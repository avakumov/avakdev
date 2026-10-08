import {
  forwardRef,
  useCallback,
  useEffect,
  useImperativeHandle,
  useRef,
} from "react";
// Legacy-сборка pdf.js содержит полифиллы для не очень новых браузеров
// (например Uint8Array.prototype.toHex), поэтому берём именно её.
import * as pdfjsLib from "pdfjs-dist/legacy/build/pdf.mjs";
import PdfWorker from "pdfjs-dist/legacy/build/pdf.worker.min.mjs?worker";
import "pdfjs-dist/web/pdf_viewer.css";

// PDF-ридер раздела «Чтение». Своего скролла нет — страницы рендерятся внутрь
// общего скролл-контейнера модалки (его ведёт BookModal), поэтому перехват
// прокрутки для времени чтения и «дошли до конца» работает как для fb2/epub.
//
// Производительность: страницы рисуются по мере приближения к области
// просмотра (IntersectionObserver) и освобождаются, когда уходят далеко —
// поэтому открытие большого PDF не блокируется. Подсветки накладываются на
// текстовый слой нужной страницы после её отрисовки.

// Один общий воркер на всё приложение. Создаём лениво и НЕ уничтожаем: терминация
// воркера с активными задачами даёт AbortException, а React StrictMode монтирует
// эффект дважды. Документ закрываем обычным task.destroy() — он не трогает наш
// общий воркер (он передан снаружи).
let sharedWorker = null;
function getSharedWorker() {
  if (!sharedWorker || sharedWorker.destroyed) {
    sharedWorker = new pdfjsLib.PDFWorker({ port: new PdfWorker() });
  }
  return sharedWorker;
}

// Смещение точки (узел, offset) в символах от начала контейнера.
function offsetOfPoint(root, node, offset) {
  const range = document.createRange();
  range.selectNodeContents(root);
  range.setEnd(node, offset);
  return range.toString().length;
}

const PdfReader = forwardRef(function PdfReader(
  { url, highlights = [], scrollerRef, layerRef, onPages, onPage, onError, onHighlightClick },
  ref,
) {
  const rootRef = useRef(null);
  const docRef = useRef(null);
  const scaleRef = useRef(1.5);
  const genRef = useRef(0); // растёт при перестройке/размонтировании
  const wrapsRef = useRef([]); // обёртки страниц (всегда есть, задают высоту)
  const renderedRef = useRef([]); // по индексу: { textLayer } | "pending" | null
  const observerRef = useRef(null);
  const highlightsRef = useRef(highlights);
  highlightsRef.current = highlights;

  // Наложить подсветки на текстовый слой одной страницы.
  const applyPageHighlights = useCallback((idx) => {
    const entry = renderedRef.current[idx];
    if (!entry || entry === "pending" || !entry.textLayer) return;
    const layer = entry.textLayer;
    for (const span of layer.querySelectorAll("span.book-hl")) {
      span.classList.remove("book-hl");
      span.style.background = "";
      delete span.dataset.color;
      delete span.dataset.hlId;
    }
    for (const h of highlightsRef.current) {
      if (h.page - 1 !== idx || h.end <= h.start) continue;
      const nodes = [];
      const walker = document.createTreeWalker(layer, NodeFilter.SHOW_TEXT);
      let acc = 0;
      for (let node = walker.nextNode(); node; node = walker.nextNode()) {
        const nodeEnd = acc + node.nodeValue.length;
        if (nodeEnd > h.start && acc < h.end) {
          const span = node.parentElement?.closest("span");
          if (span) nodes.push(span);
        }
        acc = nodeEnd;
      }
      for (const span of nodes) {
        span.classList.add("book-hl");
        span.dataset.color = h.color;
        span.dataset.hlId = String(h.id);
      }
    }
  }, []);

  // Освободить отрисованную страницу (canvas + текстовый слой), оставив обёртку.
  const releasePage = useCallback((idx) => {
    const entry = renderedRef.current[idx];
    if (!entry || entry === "pending") {
      renderedRef.current[idx] = null;
      return;
    }
    const wrap = wrapsRef.current[idx];
    if (wrap) wrap.replaceChildren();
    renderedRef.current[idx] = null;
  }, []);

  // Перейти к странице (для закладок, внутренних ссылок, продолжения чтения).
  const goToPage = useCallback(
    (page) => {
      const scroller = scrollerRef?.current;
      const el = wrapsRef.current[page - 1];
      if (!scroller || !el) return;
      const sr = scroller.getBoundingClientRect();
      const er = el.getBoundingClientRect();
      scroller.scrollTop += er.top - sr.top - 8;
    },
    [scrollerRef],
  );

  // Минимальный сервис ссылок для слоя аннотаций: внешние — в новой вкладке,
  // внутренние (dest) — переход к нужной странице.
  const linkServiceRef = useRef(null);
  const getLinkService = useCallback(() => {
    if (!linkServiceRef.current) {
      linkServiceRef.current = {
        addLinkAttributes(link, url) {
          link.href = url;
          link.target = "_blank";
          link.rel = "noopener noreferrer nofollow";
        },
        // Нужны AnnotationLayer для внутренних ссылок (dest).
        getDestinationHash(dest) {
          return typeof dest === "string" ? `#${dest}` : "#";
        },
        getAnchorUrl(hash) {
          return `#${hash}`;
        },
        goToDestination(dest) {
          const doc = docRef.current;
          if (!doc) return;
          (async () => {
            try {
              const d =
                typeof dest === "string" ? await doc.getDestination(dest) : dest;
              if (Array.isArray(d) && d[0] != null) {
                const idx = await doc.getPageIndex(d[0]);
                goToPage(idx + 1);
              }
            } catch {
              /* перейти не удалось */
            }
          })();
        },
      };
    }
    return linkServiceRef.current;
  }, [goToPage]);

  // Отрисовать страницу (canvas + текстовый слой), если ещё не отрисована.
  const ensurePage = useCallback(async (idx) => {
    if (renderedRef.current[idx]) return; // уже рисуется/отрисована
    const doc = docRef.current;
    const wrap = wrapsRef.current[idx];
    if (!doc || !wrap) return;
    const gen = genRef.current;
    renderedRef.current[idx] = "pending";
    try {
      const page = await doc.getPage(idx + 1);
      if (gen !== genRef.current || renderedRef.current[idx] !== "pending") return;
      const viewport = page.getViewport({ scale: scaleRef.current });

      const dpr = Math.min(window.devicePixelRatio || 1, 1.5);
      const canvas = document.createElement("canvas");
      canvas.width = Math.floor(viewport.width * dpr);
      canvas.height = Math.floor(viewport.height * dpr);
      canvas.style.width = `${viewport.width}px`;
      canvas.style.height = `${viewport.height}px`;
      canvas.style.display = "block";

      const textLayerDiv = document.createElement("div");
      textLayerDiv.className = "textLayer";
      // pdf.js v6 считает размеры текста из --total-scale-factor (её ставит
      // встроенный просмотрщик), а позиции — в процентах.
      textLayerDiv.style.setProperty("--scale-factor", String(scaleRef.current));
      textLayerDiv.style.setProperty("--total-scale-factor", String(scaleRef.current));

      wrap.replaceChildren(canvas, textLayerDiv);

      const ctx = canvas.getContext("2d");
      await page.render({
        canvasContext: ctx,
        viewport,
        transform: dpr !== 1 ? [dpr, 0, 0, dpr, 0, 0] : null,
      }).promise;
      const textContent = await page.getTextContent();
      const textLayer = new pdfjsLib.TextLayer({
        textContentSource: textContent,
        container: textLayerDiv,
        viewport,
      });
      await textLayer.render();
      if (gen !== genRef.current || renderedRef.current[idx] !== "pending") return;

      // Слой аннотаций: гиперссылки (внешние — в новой вкладке, внутренние —
      // переход по странице).
      try {
        const annotations = await page.getAnnotations({ intent: "display" });
        if (annotations.length) {
          const annDiv = document.createElement("div");
          annDiv.className = "annotationLayer";
          annDiv.style.setProperty("--scale-factor", String(scaleRef.current));
          annDiv.style.setProperty("--total-scale-factor", String(scaleRef.current));
          // Ссылки должны быть выше текстового слоя, иначе клики не доходят.
          annDiv.style.zIndex = "2";
          wrap.appendChild(annDiv);
          const annLayer = new pdfjsLib.AnnotationLayer({
            div: annDiv,
            page,
            viewport,
            linkService: getLinkService(),
          });
          await annLayer.render({ annotations });
        }
      } catch {
        /* слой аннотаций не критичен для чтения */
      }

      if (gen !== genRef.current || renderedRef.current[idx] !== "pending") return;
      renderedRef.current[idx] = { textLayer: textLayerDiv };
      applyPageHighlights(idx);
    } catch {
      if (renderedRef.current[idx] === "pending") {
        renderedRef.current[idx] = null;
        const w = wrapsRef.current[idx];
        if (w) w.replaceChildren();
      }
    }
  }, [applyPageHighlights, getLinkService]);

  // Перестроить разметку страниц под текущую ширину и запустить наблюдение.
  const layout = useCallback(async (doc) => {
    const root = rootRef.current;
    if (!root) return;
    const gen = ++genRef.current;
    observerRef.current?.disconnect();
    const scroller = scrollerRef?.current;
    // Запоминаем относительную позицию, чтобы не кидало в начало при перестройке.
    const ratio =
      scroller && scroller.scrollHeight
        ? scroller.scrollTop / scroller.scrollHeight
        : 0;
    root.replaceChildren();
    wrapsRef.current = [];
    renderedRef.current = [];

    const first = await doc.getPage(1);
    if (gen !== genRef.current) return;
    const base = first.getViewport({ scale: 1 });
    const avail = (root.clientWidth || window.innerWidth || 800) - 16;
    scaleRef.current = Math.min(3, Math.max(0.5, avail / base.width));
    const vp = first.getViewport({ scale: scaleRef.current });

    const frag = document.createDocumentFragment();
    for (let i = 0; i < doc.numPages; i++) {
      const wrap = document.createElement("div");
      wrap.className = "pdf-page";
      wrap.dataset.page = String(i + 1);
      wrap.style.position = "relative";
      wrap.style.width = `${vp.width}px`;
      wrap.style.height = `${vp.height}px`;
      wrap.style.margin = "0 auto 12px";
      // pdf.js считает размеры слоёв (текстового и аннотаций) через эти
      // переменные; вне .pdfViewer их надо задать самим — иначе размер
      // получается 0 и элементы (в процентах) сжимаются в точку.
      wrap.style.setProperty("--scale-factor", String(scaleRef.current));
      wrap.style.setProperty("--total-scale-factor", String(scaleRef.current));
      wrap.style.setProperty("--scale-round-x", "1px");
      wrap.style.setProperty("--scale-round-y", "1px");
      wrapsRef.current[i] = wrap;
      frag.appendChild(wrap);
    }
    root.appendChild(frag);
    if (scroller) scroller.scrollTop = ratio * scroller.scrollHeight;

    const io = new IntersectionObserver(
      (entries) => {
        for (const e of entries) {
          const idx = Number(e.target.dataset.page) - 1;
          if (e.isIntersecting) ensurePage(idx);
          else releasePage(idx);
        }
      },
      { root: scroller || null, rootMargin: "120% 0px" },
    );
    observerRef.current = io;
    wrapsRef.current.forEach((w) => io.observe(w));
  }, [ensurePage, releasePage, scrollerRef]);

  // Загрузка документа.
  useEffect(() => {
    let alive = true;
    if (!rootRef.current) return undefined;
    const task = pdfjsLib.getDocument({ url, worker: getSharedWorker() });

    (async () => {
      const doc = await task.promise;
      if (!alive) return;
      docRef.current = doc;
      onPages?.(doc.numPages);
      await layout(doc);
      if (alive) onPage?.(1);
    })().catch((err) => {
      if (!alive) return;
      console.error("PdfReader: не удалось открыть PDF", err);
      onError?.(err);
    });

    return () => {
      alive = false;
      genRef.current++;
      observerRef.current?.disconnect();
      try {
        task.destroy().catch(() => {});
      } catch {
        /* задача уже завершена */
      }
      docRef.current = null;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [url]);

  // Перестройка при заметном изменении ширины контейнера.
  useEffect(() => {
    const root = rootRef.current;
    if (!root || typeof ResizeObserver === "undefined") return undefined;
    let timer = null;
    let last = root.clientWidth;
    const ro = new ResizeObserver(() => {
      const w = root.clientWidth;
      if (Math.abs(w - last) < 8) return;
      last = w;
      clearTimeout(timer);
      timer = setTimeout(() => {
        if (docRef.current) layout(docRef.current);
      }, 250);
    });
    ro.observe(root);
    return () => {
      clearTimeout(timer);
      ro.disconnect();
    };
  }, [layout]);

  // Подсветки могли измениться — перекладываем по отрисованным страницам.
  useEffect(() => {
    for (let i = 0; i < renderedRef.current.length; i++) applyPageHighlights(i);
  }, [highlights, applyPageHighlights]);

  // Клик по подсветке — сообщаем наверх (меню «Удалить»).
  const handleClick = (e) => {
    const el = e.target.closest?.(".book-hl");
    if (!el) return;
    const rect = el.getBoundingClientRect();
    const lr = layerRef?.current?.getBoundingClientRect();
    if (!lr) return;
    onHighlightClick?.(Number(el.dataset.hlId), {
      left: rect.left - lr.left,
      top: rect.bottom - lr.top,
    });
  };

  // Текущая видимая страница при прокрутке общего скроллера.
  useEffect(() => {
    const scroller = scrollerRef?.current;
    if (!scroller) return undefined;
    const onScroll = () => {
      const top = scroller.getBoundingClientRect().top;
      let best = 1;
      for (let i = 0; i < wrapsRef.current.length; i++) {
        const el = wrapsRef.current[i];
        if (el && el.getBoundingClientRect().bottom > top + 8) {
          best = i + 1;
          break;
        }
      }
      onPage?.(best);
    };
    scroller.addEventListener("scroll", onScroll, { passive: true });
    return () => scroller.removeEventListener("scroll", onScroll);
  }, [scrollerRef, onPage]);

  useImperativeHandle(ref, () => ({
    // Перейти к странице (для закладок/продолжения чтения).
    goToPage,
    // Текущее выделение внутри одной страницы (или null).
    readSelection() {
      const s = window.getSelection();
      if (!s || s.rangeCount === 0 || s.isCollapsed) return null;
      const range = s.getRangeAt(0);
      let pageIdx = -1;
      for (let i = 0; i < wrapsRef.current.length; i++) {
        const el = wrapsRef.current[i];
        if (el && el.contains(range.startContainer) && el.contains(range.endContainer)) {
          pageIdx = i;
          break;
        }
      }
      if (pageIdx < 0) return null;
      const entry = renderedRef.current[pageIdx];
      if (!entry || entry === "pending" || !entry.textLayer) return null;
      const layer = entry.textLayer;
      const start = offsetOfPoint(layer, range.startContainer, range.startOffset);
      const end = offsetOfPoint(layer, range.endContainer, range.endOffset);
      if (end <= start) return null;
      const r = range.getBoundingClientRect();
      const lr = layerRef?.current?.getBoundingClientRect();
      if (!lr) return null;
      return {
        page: pageIdx + 1,
        start,
        end,
        text: s.toString(),
        left: r.left - lr.left,
        top: r.top - lr.top,
        bottom: r.bottom - lr.top,
        width: r.width,
        layerWidth: layerRef.current.clientWidth,
      };
    },
  }));

  return (
    <div ref={rootRef} className="pdf-reader relative w-full" onClick={handleClick} />
  );
});

export default PdfReader;
