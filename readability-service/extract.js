import { Readability } from "@mozilla/readability"
import { JSDOM } from "jsdom"

export async function extractArticle(url, html) {
    const dom = new JSDOM(html, { url }) // pass the url so relative links resolve

    const reader = new Readability(dom.window.document)
    const article = reader.parse()
    if (!article) {
        throw new Error("Failed to parse article")
    }

    return article.content
}
