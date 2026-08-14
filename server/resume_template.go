package main

// resumeTemplateCSS содержит CSS-шаблон резюме в одну человекочитаемую строку.
// Он передаётся DeepSeek в системном сообщении как образец оформления,
// чтобы сгенерированное резюме точно соответствовало нашему дизайну.
//
// Это формат А4, ориентированный на печать: страница сама подстраивается,
// а при печати в PDF браузер корректно разобьёт документ на страницы.
func resumeTemplateCSS() string {
	return `<style>
  :root {
    --accent: #2563eb;
    --ink: #1f2937;
    --muted: #6b7280;
    --line: #e5e7eb;
  }
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body {
    font-family: "Segoe UI", system-ui, Roboto, Arial, sans-serif;
    color: var(--ink);
    background: #f3f4f6;
    line-height: 1.5;
  }
  .page {
    max-width: 800px;
    margin: 24px auto;
    background: #ffffff;
    padding: 40px 48px;
    box-shadow: 0 1px 3px rgba(0,0,0,.08);
    border-radius: 8px;
  }
  .header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 24px;
    margin-bottom: 20px;
  }
  .header .info { flex: 1; min-width: 0; }
  .header .photo-wrap { flex-shrink: 0; }
  .photo {
    display: block;
    width: 140px;
    height: 170px;
    object-fit: cover;
    border-radius: 0;
    margin: 0;
    border: 1px solid #d1d5db;
  }
  h1 {
    font-size: 30px;
    font-weight: 700;
    color: var(--ink);
    margin-bottom: 2px;
  }
  .role { font-size: 16px; color: var(--accent); font-weight: 600; margin-bottom: 12px; }
  .contacts { color: var(--muted); font-size: 14px; display: flex; flex-wrap: wrap; gap: 4px 16px; }
  .section { margin-top: 20px; }
  .section h2 {
    font-size: 14px;
    text-transform: uppercase;
    letter-spacing: .08em;
    color: var(--accent);
    border-bottom: 2px solid var(--line);
    padding-bottom: 6px;
    margin-bottom: 12px;
  }
  .section p { margin-bottom: 8px; }
  ul { margin: 0 0 8px 18px; }
  li { margin-bottom: 4px; }
  .job { margin-bottom: 14px; }
  .job .head { display: flex; justify-content: space-between; flex-wrap: wrap; font-weight: 600; }
  .job .head .dates { font-weight: 400; color: var(--muted); font-style: italic; }
  .skill-tags { display: flex; flex-wrap: wrap; gap: 6px; }
  .skill-tags span {
    background: #eff6ff;
    color: var(--accent);
    border: 1px solid #bfdbfe;
    border-radius: 999px;
    padding: 2px 10px;
    font-size: 13px;
  }
  @media (max-width: 640px) {
    .page { padding: 24px 20px; margin: 8px auto; }
    .header { flex-direction: column; align-items: flex-start; gap: 16px; }
    .header .photo-wrap { order: -1; align-self: center; }
  }
  @media print {
    body { background: #fff; }
    .page { max-width: none; margin: 0; padding: 0; box-shadow: none; border-radius: 0; }
    .section { break-inside: avoid; }
  }
</style>`
}

// resumeTemplateHead возвращает открывающую часть HTML-документа резюме:
// DOCTYPE, head с charset/viewport и встроенным CSS-шаблоном.
func resumeTemplateHead() string {
	return "<!DOCTYPE html>\n<html lang=\"ru\">\n<head>\n<meta charset=\"UTF-8\">\n" +
		"<meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\">\n" +
		"<title>Резюме</title>\n" +
		resumeTemplateCSS() + "\n</head>\n<body>\n<div class=\"page\">\n"
}

// resumeTemplateFoot возвращает закрывающую часть HTML-документа резюме.
func resumeTemplateFoot() string {
	return "\n</div>\n</body>\n</html>"
}

// resumeTemplateBodyAsExample возвращает пример структуры тела резюме
// (пустые разделы с плейсхолдерами), который передаётся DeepSeek,
// чтобы модель вписала сгенерированный контент в наш шаблон.
func resumeTemplateBodyAsExample() string {
	return `<div class="header">
  <div class="info">
    <h1>ИМЯ ФАМИЛИЯ</h1>
    <div class="role">ДОЛЖНОСТЬ</div>
    <div class="contacts">
      <span>email@example.com</span>
      <span>+7 (000) 000-00-00</span>
      <span>Город</span>
      <span>github.com/username</span>
    </div>
  </div>
  <div class="photo-wrap"><img id="resume-photo" class="photo" alt="Фото"></div>
</div>

<div class="section">
  <h2>О себе</h2>
  <p>Краткое описание соискателя.</p>
</div>

<div class="section">
  <h2>Навыки</h2>
  <div class="skill-tags">
    <span>Навык 1</span>
    <span>Навык 2</span>
    <span>Навык 3</span>
  </div>
</div>

<div class="section">
  <h2>Опыт работы</h2>
  <div class="job">
    <div class="head"><span>Компания — должность</span><span class="dates">2020 — 2024</span></div>
    <ul>
      <li>Достижение или обязанность.</li>
      <li>Достижение или обязанность.</li>
    </ul>
  </div>
</div>

<div class="section">
  <h2>Образование</h2>
  <p><strong>Университет</strong> — специальность, год окончания.</p>
</div>

<div class="section">
  <h2>Достижения</h2>
  <ul>
    <li>Конкретное достижение.</li>
  </ul>
</div>`
}
