import { useEffect, useState } from 'react'

function App() {
  const [health, setHealth] = useState(null)
  const [message, setMessage] = useState(null)

  useEffect(() => {
    // Все запросы идут на тот же хост, что отдал страницу,
    // т.е. через Go-сервер (в dev — через vite-прокси на него).
    fetch('/api/health')
      .then((res) => res.json())
      .then(setHealth)
      .catch((err) => setHealth({ error: String(err) }))

    fetch('/api/message')
      .then((res) => res.json())
      .then(setMessage)
      .catch((err) => setMessage({ error: String(err) }))
  }, [])

  return (
    <main className="container">
      <h1>🚀 Go (Gin) + React</h1>
      <p className="subtitle">
        Фронтенд отдаётся Go-сервером, данные приходят с <code>/api</code>
      </p>

      <section className="card">
        <h2>Health</h2>
        <pre>{JSON.stringify(health, null, 2)}</pre>
      </section>

      <section className="card">
        <h2>Message</h2>
        <pre>{JSON.stringify(message, null, 2)}</pre>
      </section>
    </main>
  )
}

export default App
