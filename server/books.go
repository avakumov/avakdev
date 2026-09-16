package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	stdhtml "html"
	"io"
	"log"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	xhtml "golang.org/x/net/html"
	"golang.org/x/text/encoding/charmap"
)

// Раздел «Чтение»: пользователь загружает книгу в формате FB2 или EPUB,
// сервер преобразует её в HTML (с картинками внутри), хранит в таблице books
// и отдаёт готовый HTML для чтения.

// Book — книга пользователя (HTML-версия текста).
type Book struct {
	ID      int    `json:"id"`
	Title   string `json:"title"`
	Author  string `json:"author"`
	Format  string `json:"format"`
	Created string `json:"created"`
	// FinishedAt — когда книга отмечена прочитанной (пусто — не прочитана).
	FinishedAt string `json:"finished_at"`
	// HTML — сконвертированный текст; в списке не отдаётся (omitempty).
	HTML string `json:"html,omitempty"`
}

// maxBookBytes — предельный размер загружаемого файла книги.
const maxBookBytes = 40 << 20

// bookCreatedExpr — единый формат времени создания (RFC3339, UTC).
const bookCreatedExpr = `to_char(created AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')`

// bookFinishedExpr — отметка о прочтении (RFC3339, UTC; пусто — не прочитана).
const bookFinishedExpr = `COALESCE(to_char(finished_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'), '')`

// handleListBooks возвращает книги текущего пользователя (без текста).
// Непрочитанные идут первыми, прочитанные — в конце списка.
func handleListBooks(c *gin.Context) {
	sessData, _ := c.MustGet("session").(session)
	rows, err := db.Query(context.Background(),
		`SELECT id, title, author, format, `+bookCreatedExpr+`, `+bookFinishedExpr+`
		 FROM books WHERE username = $1
		 ORDER BY (finished_at IS NOT NULL), id DESC`,
		sessData.username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось загрузить книги"})
		return
	}
	defer rows.Close()

	out := make([]Book, 0)
	for rows.Next() {
		var b Book
		if err := rows.Scan(&b.ID, &b.Title, &b.Author, &b.Format, &b.Created, &b.FinishedAt); err == nil {
			out = append(out, b)
		}
	}
	c.JSON(http.StatusOK, out)
}

// handleGetBook возвращает книгу вместе с HTML-текстом.
func handleGetBook(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID книги"})
		return
	}
	sessData, _ := c.MustGet("session").(session)

	var b Book
	err = db.QueryRow(context.Background(),
		`SELECT id, title, author, format, html, `+bookCreatedExpr+`, `+bookFinishedExpr+`
		 FROM books WHERE id = $1 AND username = $2`,
		id, sessData.username).
		Scan(&b.ID, &b.Title, &b.Author, &b.Format, &b.HTML, &b.Created, &b.FinishedAt)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Книга не найдена"})
		return
	}
	c.JSON(http.StatusOK, b)
}

// handleUploadBook принимает файл книги (multipart/form-data, поле «file»),
// конвертирует его в HTML и сохраняет.
func handleUploadBook(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Выберите файл книги (fb2 или epub)"})
		return
	}
	f, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Не удалось открыть файл"})
		return
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, maxBookBytes+1))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Не удалось прочитать файл"})
		return
	}
	if len(data) > maxBookBytes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Файл слишком большой (макс. 40 МБ)"})
		return
	}

	format, title, author, bookHTML, err := convertBook(fileHeader.Filename, data)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(bookHTML) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Не удалось извлечь текст книги"})
		return
	}

	sessData, _ := c.MustGet("session").(session)
	var b Book
	err = db.QueryRow(context.Background(),
		`INSERT INTO books (username, title, author, format, html)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, `+bookCreatedExpr,
		sessData.username, title, author, format, bookHTML).
		Scan(&b.ID, &b.Created)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось сохранить книгу"})
		return
	}
	b.Title, b.Author, b.Format = title, author, format
	c.JSON(http.StatusOK, b)
}

// handleDeleteBook удаляет книгу пользователя.
func handleDeleteBook(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID книги"})
		return
	}
	sessData, _ := c.MustGet("session").(session)
	tag, err := db.Exec(context.Background(),
		`DELETE FROM books WHERE id = $1 AND username = $2`, id, sessData.username)
	if err != nil || tag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Книга не найдена"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// handleSetBookFinished отмечает книгу прочитанной (finished=true) или
// возвращает её в чтение (finished=false).
func handleSetBookFinished(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID книги"})
		return
	}
	var req struct {
		Finished bool `json:"finished"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
		return
	}
	sessData, _ := c.MustGet("session").(session)

	var finishedAt string
	err = db.QueryRow(context.Background(),
		`UPDATE books
		 SET finished_at = CASE WHEN $3 THEN now() ELSE NULL END
		 WHERE id = $1 AND username = $2
		 RETURNING `+bookFinishedExpr,
		id, sessData.username, req.Finished).Scan(&finishedAt)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Книга не найдена"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": id, "finished_at": finishedAt})
}

// convertBook определяет формат файла и преобразует книгу в HTML.
// Возвращает формат ("fb2" | "epub"), название, автора и HTML.
func convertBook(filename string, data []byte) (format, title, author, body string, err error) {
	if isZip(data) {
		zr, zerr := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if zerr != nil {
			return "", "", "", "", errors.New("не удалось прочитать архив книги")
		}
		// EPUB опознаём по META-INF/container.xml.
		if _, ok := zipEntry(zr, "META-INF/container.xml"); ok {
			t, a, h, e := epubToHTML(zr)
			return "epub", t, a, h, e
		}
		// Иначе это fb2.zip — внутри лежит .fb2.
		for _, f := range zr.File {
			if strings.HasSuffix(strings.ToLower(f.Name), ".fb2") {
				raw, e := readZipEntry(f)
				if e != nil {
					return "", "", "", "", errors.New("не удалось прочитать FB2 из архива")
				}
				t, a, h, e := fb2ToHTML(raw)
				return "fb2", t, a, h, e
			}
		}
		return "", "", "", "", errors.New("в архиве нет ни EPUB, ни FB2 книги")
	}

	if bytes.Contains(data[:min(len(data), 4096)], []byte("<FictionBook")) ||
		strings.HasSuffix(strings.ToLower(filename), ".fb2") {
		t, a, h, e := fb2ToHTML(data)
		return "fb2", t, a, h, e
	}
	return "", "", "", "", errors.New("поддерживаются только файлы FB2 и EPUB")
}

// isZip — проверка по сигнатуре ZIP-архива.
func isZip(data []byte) bool {
	return len(data) >= 4 && data[0] == 'P' && data[1] == 'K' &&
		(data[2] == 3 || data[2] == 5 || data[2] == 7)
}

// zipEntry ищет файл в архиве (регистронезависимо).
func zipEntry(zr *zip.Reader, name string) (*zip.File, bool) {
	for _, f := range zr.File {
		if strings.EqualFold(f.Name, name) {
			return f, true
		}
	}
	return nil, false
}

// readZipEntry читает содержимое файла из архива.
func readZipEntry(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(io.LimitReader(rc, maxBookBytes))
}

// ---- FB2 ----

// fb2Meta — метаданные книги (<description><title-info>).
type fb2Meta struct {
	TitleInfo struct {
		BookTitle string `xml:"book-title"`
		Authors   []struct {
			FirstName string `xml:"first-name"`
			LastName  string `xml:"last-name"`
			Nickname  string `xml:"nickname"`
		} `xml:"author"`
	} `xml:"title-info"`
}

// trimBOM убирает UTF-8 BOM (мешает разбору XML).
func trimBOM(data []byte) []byte {
	return bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
}

// fb2Decoder создаёт XML-декодер с поддержкой типовых кодировок FB2.
func fb2Decoder(data []byte) *xml.Decoder {
	dec := xml.NewDecoder(bytes.NewReader(trimBOM(data)))
	dec.CharsetReader = func(charset string, input io.Reader) (io.Reader, error) {
		switch strings.ToLower(strings.TrimSpace(charset)) {
		case "", "utf-8", "utf8":
			return input, nil
		case "windows-1251", "cp1251":
			return charmap.Windows1251.NewDecoder().Reader(input), nil
		case "koi8-r", "koi8r":
			return charmap.KOI8R.NewDecoder().Reader(input), nil
		case "windows-1252", "cp1252":
			return charmap.Windows1252.NewDecoder().Reader(input), nil
		case "iso-8859-1", "latin1":
			return charmap.ISO8859_1.NewDecoder().Reader(input), nil
		}
		return nil, fmt.Errorf("неизвестная кодировка книги: %s", charset)
	}
	return dec
}

// fb2ToHTML преобразует FB2 (XML) в HTML: метаданные, картинки из <binary>
// (вставляются как data URL) и текст тела книги.
func fb2ToHTML(data []byte) (title, author, body string, err error) {
	dec := fb2Decoder(data)
	images := map[string]string{}

	// Первый проход: метаданные и картинки.
	for {
		tok, terr := dec.Token()
		if terr == io.EOF {
			break
		}
		if terr != nil {
			return "", "", "", errors.New("не удалось разобрать FB2: " + terr.Error())
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		switch strings.ToLower(start.Name.Local) {
		case "description":
			var meta fb2Meta
			if derr := dec.DecodeElement(&meta, &start); derr == nil {
				title = strings.TrimSpace(meta.TitleInfo.BookTitle)
				author = fb2Authors(meta)
			}
		case "binary":
			var bin struct {
				ID          string `xml:"id,attr"`
				ContentType string `xml:"content-type,attr"`
				Data        string `xml:",chardata"`
			}
			if derr := dec.DecodeElement(&bin, &start); derr == nil && bin.ID != "" {
				ctype := strings.TrimSpace(bin.ContentType)
				if ctype == "" {
					ctype = "image/jpeg"
				}
				images[bin.ID] = "data:" + ctype + ";base64," +
					strings.Join(strings.Fields(bin.Data), "")
			}
		}
	}

	// Второй проход: тело книги.
	dec = fb2Decoder(data)
	var sb strings.Builder
	titleDepth := 0
	for {
		tok, terr := dec.Token()
		if terr == io.EOF {
			break
		}
		if terr != nil {
			return "", "", "", errors.New("не удалось разобрать FB2: " + terr.Error())
		}
		switch t := tok.(type) {
		case xml.StartElement:
			tag := strings.ToLower(t.Name.Local)
			switch tag {
			case "description", "binary":
				if serr := dec.Skip(); serr != nil {
					return "", "", "", serr
				}
			case "body":
				sb.WriteString(`<div class="fb2-body">`)
			case "section":
				sb.WriteString("<section>")
			case "title":
				titleDepth++
				sb.WriteString(`<h3 class="book-heading">`)
			case "subtitle":
				sb.WriteString("<h4>")
			case "p":
				if titleDepth == 0 {
					sb.WriteString("<p>")
				}
			case "v":
				sb.WriteString(`<p class="verse">`)
			case "empty-line":
				sb.WriteString("<br>")
			case "emphasis":
				sb.WriteString("<em>")
			case "strong":
				sb.WriteString("<strong>")
			case "strikethrough":
				sb.WriteString("<s>")
			case "sub":
				sb.WriteString("<sub>")
			case "sup":
				sb.WriteString("<sup>")
			case "code":
				sb.WriteString("<code>")
			case "cite":
				sb.WriteString("<blockquote>")
			case "epigraph":
				sb.WriteString(`<div class="epigraph">`)
			case "poem":
				sb.WriteString(`<div class="poem">`)
			case "stanza":
				sb.WriteString(`<div class="stanza">`)
			case "text-author":
				sb.WriteString(`<p class="text-author">`)
			case "table":
				sb.WriteString("<table>")
			case "tr":
				sb.WriteString("<tr>")
			case "th":
				sb.WriteString("<th>")
			case "td":
				sb.WriteString("<td>")
			case "a":
				href := xmlAttr(t, "href")
				sb.WriteString(`<a href="` + stdhtml.EscapeString(href) + `">`)
			case "image":
				ref := strings.TrimPrefix(xmlAttr(t, "href"), "#")
				if src, ok := images[ref]; ok {
					sb.WriteString(`<img src="` + src + `" alt="">`)
				}
				if serr := dec.Skip(); serr != nil {
					return "", "", "", serr
				}
			}
		case xml.EndElement:
			tag := strings.ToLower(t.Name.Local)
			switch tag {
			case "body":
				sb.WriteString("</div>")
			case "section":
				sb.WriteString("</section>")
			case "title":
				titleDepth--
				sb.WriteString("</h3>")
			case "subtitle":
				sb.WriteString("</h4>")
			case "p":
				if titleDepth == 0 {
					sb.WriteString("</p>")
				}
			case "v":
				sb.WriteString("</p>")
			case "emphasis":
				sb.WriteString("</em>")
			case "strong":
				sb.WriteString("</strong>")
			case "strikethrough":
				sb.WriteString("</s>")
			case "sub":
				sb.WriteString("</sub>")
			case "sup":
				sb.WriteString("</sup>")
			case "code":
				sb.WriteString("</code>")
			case "cite":
				sb.WriteString("</blockquote>")
			case "epigraph":
				sb.WriteString("</div>")
			case "poem":
				sb.WriteString("</div>")
			case "stanza":
				sb.WriteString("</div>")
			case "text-author":
				sb.WriteString("</p>")
			case "table":
				sb.WriteString("</table>")
			case "tr":
				sb.WriteString("</tr>")
			case "th":
				sb.WriteString("</th>")
			case "td":
				sb.WriteString("</td>")
			case "a":
				sb.WriteString("</a>")
			}
		case xml.CharData:
			sb.WriteString(stdhtml.EscapeString(string(t)))
		}
	}

	if strings.TrimSpace(title) == "" {
		title = "Без названия"
	}
	return title, author, sb.String(), nil
}

// fb2Authors собирает авторов в строку «Имя Фамилия, Имя Фамилия».
func fb2Authors(meta fb2Meta) string {
	names := make([]string, 0, len(meta.TitleInfo.Authors))
	for _, a := range meta.TitleInfo.Authors {
		parts := make([]string, 0, 2)
		if s := strings.TrimSpace(a.FirstName); s != "" {
			parts = append(parts, s)
		}
		if s := strings.TrimSpace(a.LastName); s != "" {
			parts = append(parts, s)
		}
		name := strings.Join(parts, " ")
		if name == "" {
			name = strings.TrimSpace(a.Nickname)
		}
		if name != "" {
			names = append(names, name)
		}
	}
	return strings.Join(names, ", ")
}

// xmlAttr возвращает значение атрибута по локальному имени.
func xmlAttr(el xml.StartElement, name string) string {
	for _, a := range el.Attr {
		if strings.EqualFold(a.Name.Local, name) {
			return a.Value
		}
	}
	return ""
}

// ---- EPUB ----

// epubPackage — разобранный OPF-файл (метаданные, манифест, порядок глав).
type epubPackage struct {
	Metadata struct {
		Titles   []string `xml:"title"`
		Creators []string `xml:"creator"`
	} `xml:"metadata"`
	Manifest struct {
		Items []struct {
			ID        string `xml:"id,attr"`
			Href      string `xml:"href,attr"`
			MediaType string `xml:"media-type,attr"`
		} `xml:"item"`
	} `xml:"manifest"`
	Spine struct {
		Refs []struct {
			IDRef string `xml:"idref,attr"`
		} `xml:"itemref"`
	} `xml:"spine"`
}

// epubToHTML собирает HTML книги из XHTML-глав EPUB (в порядке spine),
// очищая разметку и вставляя картинки как data URL.
func epubToHTML(zr *zip.Reader) (title, author, body string, err error) {
	containerFile, ok := zipEntry(zr, "META-INF/container.xml")
	if !ok {
		return "", "", "", errors.New("повреждён EPUB: нет META-INF/container.xml")
	}
	containerData, err := readZipEntry(containerFile)
	if err != nil {
		return "", "", "", errors.New("не удалось прочитать контейнер EPUB")
	}
	containerData = trimBOM(containerData)
	var container struct {
		Rootfiles []struct {
			FullPath string `xml:"full-path,attr"`
		} `xml:"rootfiles>rootfile"`
	}
	if err := xml.Unmarshal(containerData, &container); err != nil ||
		len(container.Rootfiles) == 0 {
		return "", "", "", errors.New("повреждён EPUB: не найден OPF-файл")
	}

	opfPath := strings.TrimSpace(container.Rootfiles[0].FullPath)
	opfFile, ok := zipEntry(zr, opfPath)
	if !ok {
		return "", "", "", errors.New("повреждён EPUB: OPF-файл отсутствует")
	}
	opfData, err := readZipEntry(opfFile)
	if err != nil {
		return "", "", "", errors.New("не удалось прочитать OPF-файл")
	}
	opfData = trimBOM(opfData)
	var pkg epubPackage
	if err := xml.Unmarshal(opfData, &pkg); err != nil {
		return "", "", "", errors.New("не удалось разобрать OPF-файл")
	}

	if len(pkg.Metadata.Titles) > 0 {
		title = strings.TrimSpace(pkg.Metadata.Titles[0])
	}
	if len(pkg.Metadata.Creators) > 0 {
		author = strings.TrimSpace(pkg.Metadata.Creators[0])
	}

	manifest := make(map[string]struct{ Href, MediaType string }, len(pkg.Manifest.Items))
	for _, it := range pkg.Manifest.Items {
		manifest[it.ID] = struct{ Href, MediaType string }{it.Href, it.MediaType}
	}

	base := path.Dir(opfPath)
	if base == "." {
		base = ""
	}

	chapters := make([]string, 0, len(pkg.Spine.Refs))
	used := map[string]bool{}
	addChapter := func(full string) bool {
		if full == "" || used[full] {
			return false
		}
		file, ok := zipEntry(zr, full)
		if !ok {
			return false
		}
		raw, err := readZipEntry(file)
		if err != nil {
			return false
		}
		text := xhtmlToCleanHTML(raw, path.Dir(full), zr)
		if strings.TrimSpace(text) == "" {
			return false
		}
		used[full] = true
		chapters = append(chapters, text)
		return true
	}

	// 1) Главы в порядке spine.
	for _, ref := range pkg.Spine.Refs {
		if item, ok := manifest[ref.IDRef]; ok {
			addChapter(resolveBookHref(base, item.Href))
		}
	}
	// 2) Фолбэк: все элементы манифеста (если spine пуст или не совпал).
	if len(chapters) == 0 {
		for _, it := range pkg.Manifest.Items {
			addChapter(resolveBookHref(base, it.Href))
		}
	}
	// 3) Фолбэк: все XHTML/HTML архива (nav/toc — в самом конце).
	if len(chapters) == 0 {
		names := make([]string, 0, len(zr.File))
		for _, f := range zr.File {
			low := strings.ToLower(f.Name)
			if !strings.HasSuffix(low, ".xhtml") &&
				!strings.HasSuffix(low, ".html") &&
				!strings.HasSuffix(low, ".htm") {
				continue
			}
			if strings.Contains(low, "nav") || strings.Contains(low, "toc") {
				continue
			}
			names = append(names, f.Name)
		}
		sort.Strings(names)
		for _, n := range names {
			addChapter(n)
		}
		if len(chapters) == 0 {
			for _, f := range zr.File {
				low := strings.ToLower(f.Name)
				if strings.HasSuffix(low, ".xhtml") ||
					strings.HasSuffix(low, ".html") ||
					strings.HasSuffix(low, ".htm") {
					addChapter(f.Name)
				}
			}
		}
	}

	log.Printf("EPUB: извлечено глав: %d (файлов в архиве: %d, spine: %d)",
		len(chapters), len(zr.File), len(pkg.Spine.Refs))
	if len(chapters) == 0 {
		return title, author, "",
			errors.New("в EPUB не найдены XHTML-главы — возможно, книга защищена DRM")
	}

	var sb strings.Builder
	for _, ch := range chapters {
		sb.WriteString(`<section class="chapter">`)
		sb.WriteString(ch)
		sb.WriteString(`</section>`)
	}

	if strings.TrimSpace(title) == "" {
		title = "Без названия"
	}
	return title, author, sb.String(), nil
}

// resolveBookHref приводит ссылку внутри EPUB к пути в архиве.
func resolveBookHref(base, href string) string {
	href = strings.TrimSpace(href)
	if i := strings.IndexByte(href, '#'); i >= 0 {
		href = href[:i]
	}
	if u, err := url.PathUnescape(href); err == nil {
		href = u
	}
	// Абсолютные пути внутри архива: «/OEBPS/Text/ch.xhtml».
	if strings.HasPrefix(href, "/") {
		return path.Clean(strings.TrimPrefix(href, "/"))
	}
	if base == "" {
		return path.Clean(href)
	}
	return path.Clean(path.Join(base, href))
}

// voidHTMLTags — теги без закрывающей пары (в HTML).
var voidHTMLTags = map[string]bool{
	"area": true, "base": true, "br": true, "col": true, "embed": true,
	"hr": true, "img": true, "input": true, "link": true, "meta": true,
	"param": true, "source": true, "track": true, "wbr": true,
}

// Регулярки нормализации XHTML.
var (
	xmlDeclRe   = regexp.MustCompile(`(?is)<\?xml[^>]*\?>`)
	headBlockRe = regexp.MustCompile(`(?is)<head\b[^>]*>.*?</head\s*>`)
	selfCloseRe = regexp.MustCompile(`(?is)<([a-zA-Z][a-zA-Z0-9:]*)((?:[^>"']|"[^"]*"|'[^']*')*?)/>`)
	svgBlockRe  = regexp.MustCompile(`(?is)<svg\b.*?</svg\s*>`)
	imgHrefRe   = regexp.MustCompile(`(?is)(?:xlink:)?href\s*=\s*"([^"]+)"`)
)

// normalizeXHTML приводит XHTML-главу EPUB к «обычному» HTML: убирает
// XML-декларацию и <head>, разворачивает самозакрытые теги (<title/>, <div/>)
// в пары. Без этого HTML-парсер уходит в RCDATA у <title> и текст главы
// пропадает.
func normalizeXHTML(data []byte) []byte {
	s := string(data)
	s = xmlDeclRe.ReplaceAllString(s, "")
	s = headBlockRe.ReplaceAllString(s, "")
	// Обложки в EPUB часто завёрнуты в <svg><image xlink:href=…/>.
	s = svgBlockRe.ReplaceAllStringFunc(s, func(m string) string {
		if h := imgHrefRe.FindStringSubmatch(m); h != nil {
			return `<img src="` + h[1] + `" alt="">`
		}
		return ""
	})
	s = selfCloseRe.ReplaceAllStringFunc(s, func(m string) string {
		parts := selfCloseRe.FindStringSubmatch(m)
		if parts == nil {
			return m
		}
		name := parts[1]
		if voidHTMLTags[strings.ToLower(name)] {
			return m // br/img и т. п. в HTML самозакрытые и так
		}
		return "<" + name + parts[2] + "></" + name + ">"
	})
	return []byte(s)
}

// xhtmlToCleanHTML очищает XHTML-главу EPUB: убирает скрипты и стили,
// оставляет безопасные теги, а картинки вставляет как data URL.
func xhtmlToCleanHTML(data []byte, baseDir string, zr *zip.Reader) string {
	doc, err := xhtml.Parse(bytes.NewReader(normalizeXHTML(trimBOM(data))))
	if err != nil {
		return ""
	}
	// Обычно контент в <body>; если его нет — берём весь документ.
	root := findHTMLNode(doc, "body")
	if root == nil {
		root = doc
	}

	resolveImg := func(src string) (string, bool) {
		full := resolveBookHref(baseDir, src)
		file, ok := zipEntry(zr, full)
		if !ok {
			return "", false
		}
		raw, err := readZipEntry(file)
		if err != nil {
			return "", false
		}
		return "data:" + mimeByExt(full) + ";base64," +
			base64.StdEncoding.EncodeToString(raw), true
	}

	var sb strings.Builder
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		renderCleanNode(child, &sb, resolveImg)
	}
	return sb.String()
}

// findHTMLNode ищет первый элемент с указанным тегом.
func findHTMLNode(n *xhtml.Node, tag string) *xhtml.Node {
	if n.Type == xhtml.ElementNode && n.Data == tag {
		return n
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if found := findHTMLNode(child, tag); found != nil {
			return found
		}
	}
	return nil
}

// allowedBookTags — теги, которые оставляем в HTML книги.
var allowedBookTags = map[string]bool{
	"p": true, "div": true, "span": true, "section": true, "article": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"em": true, "strong": true, "i": true, "b": true, "u": true, "s": true,
	"sub": true, "sup": true, "small": true, "cite": true, "code": true,
	"pre": true, "blockquote": true, "br": true, "hr": true,
	"ul": true, "ol": true, "li": true, "dl": true, "dt": true, "dd": true,
	"table": true, "thead": true, "tbody": true, "tfoot": true,
	"tr": true, "td": true, "th": true, "caption": true,
	"figure": true, "figcaption": true, "img": true, "a": true,
}

// droppedBookTags — теги, содержимое которых не нужно вообще.
var droppedBookTags = map[string]bool{
	"script": true, "style": true, "head": true, "title": true, "meta": true,
	"link": true, "iframe": true, "object": true, "embed": true,
	"noscript": true, "svg": true, "video": true, "audio": true, "canvas": true,
}

// voidBookTags — теги без закрывающей пары.
var voidBookTags = map[string]bool{"br": true, "hr": true, "img": true}

// renderCleanNode рекурсивно пишет безопасный HTML.
func renderCleanNode(
	n *xhtml.Node,
	sb *strings.Builder,
	resolveImg func(string) (string, bool),
) {
	switch n.Type {
	case xhtml.TextNode:
		sb.WriteString(stdhtml.EscapeString(n.Data))
	case xhtml.ElementNode:
		tag := strings.ToLower(n.Data)
		if droppedBookTags[tag] {
			return
		}
		if !allowedBookTags[tag] {
			// Неизвестный тег — оставляем только содержимое.
			for child := n.FirstChild; child != nil; child = child.NextSibling {
				renderCleanNode(child, sb, resolveImg)
			}
			return
		}
		if tag == "img" {
			src := htmlAttr(n, "src")
			dataURL, ok := resolveImg(src)
			if !ok {
				return // картинку не нашли — тег не выводим
			}
			alt := stdhtml.EscapeString(htmlAttr(n, "alt"))
			sb.WriteString(`<img src="` + dataURL + `" alt="` + alt + `">`)
			return
		}

		sb.WriteString("<" + tag)
		switch tag {
		case "a":
			href := htmlAttr(n, "href")
			if safeBookHref(href) {
				sb.WriteString(` href="` + stdhtml.EscapeString(href) + `"`)
			}
		case "td", "th":
			for _, attr := range []string{"colspan", "rowspan"} {
				if v := htmlAttr(n, attr); v != "" {
					sb.WriteString(` ` + attr + `="` + stdhtml.EscapeString(v) + `"`)
				}
			}
		}
		sb.WriteString(">")

		if voidBookTags[tag] {
			return
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			renderCleanNode(child, sb, resolveImg)
		}
		sb.WriteString("</" + tag + ">")
	}
}

// htmlAttr возвращает значение атрибута узла (без учёта регистра).
func htmlAttr(n *xhtml.Node, name string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, name) {
			return a.Val
		}
	}
	return ""
}

// safeBookHref — разрешаем только безопасные ссылки.
func safeBookHref(href string) bool {
	h := strings.TrimSpace(strings.ToLower(href))
	if h == "" {
		return false
	}
	return strings.HasPrefix(h, "http://") ||
		strings.HasPrefix(h, "https://") ||
		strings.HasPrefix(h, "mailto:") ||
		strings.HasPrefix(h, "#") ||
		strings.HasPrefix(h, "note") ||
		strings.HasPrefix(h, "footnote")
}

// mimeByExt — MIME-тип картинки по расширению файла.
func mimeByExt(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".bmp":
		return "image/bmp"
	default:
		return "image/jpeg"
	}
}
