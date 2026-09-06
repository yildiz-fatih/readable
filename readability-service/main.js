import express from "express"
import { extractArticle } from "./extract.js"

const app = express()
app.use(express.json({ limit: "10mb" }))

app.post("/extract", async (req, res) => {
    if (!req.body || !req.body.url || !req.body.html) {
        return res.status(400).json({ error: "Missing 'url' or 'html' in request body" })
    }
    const url = req.body.url
    const html = req.body.html

    try {
        const article = await extractArticle(url, html)
        res.set('Content-Type', 'text/html')
        res.send(article)
    } catch (err) {
        console.error(err)
        res.status(500).json({ error: `Failed to extract article: ${err.message}` })
    }
})

const port = process.env.READABILITY_SERVICE_PORT || 8081
app.listen(port, () => {
    console.log(`starting server on ${port}`)
})
