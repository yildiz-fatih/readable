async function postJSON(url, body) {
    const response = await fetch(url, {
        method: "POST",
        headers: {
            "Content-Type": "application/json"
        },
        body: JSON.stringify(body)
    })

    if (!response.ok) {
        throw new Error(`HTTP error: ${response.status}`)
    }

    return {
        "status": response.status,
        "body": await response.json()
    }
}

function sleep(seconds) {
    return new Promise((resolve) => setTimeout(function () { resolve() }, seconds * 1000));
}

const API_BASE_URL = "http://localhost:8080"

const statusCard = document.getElementById("status-card")

const form = document.getElementById("readable-form")
const submitButton = document.getElementById("submit-button")

form.addEventListener("submit", async function (event) {
    event.preventDefault()
    submitButton.disabled = true

    const url = document.getElementById("url").value
    const format = document.getElementById("format").value

    let postResult
    try {
        postResult = await postJSON(`${API_BASE_URL}/readables`, { "url": url, "format": format })
    } catch (error) {
        // TODO: make this red
        statusCard.textContent = "error from server"
        submitButton.disabled = false
        return
    }

    if (postResult.status == 202) {
        // TODO: make this yellow
        statusCard.textContent = "working the magic..."
    } else {
        // TODO: make this red
        statusCard.textContent = "Houston, we've had a problem here..."
        return
    }

    const jobID = postResult.body.id

    while (true) {
        const getResponse = await fetch(`${API_BASE_URL}/readables/${jobID}`)
        if (!getResponse.ok) {
            // TODO: not sure what to do here
            console.error(`HTTP error: ${getResponse.status}`)
        }

        const getResult = await getResponse.json()

        if (getResult.status == "pending") {
            statusCard.textContent = "working the magic..."
        } else if (getResult.status == "failed") {
            statusCard.textContent = "Houston, we've had a problem here..."
            submitButton.disabled = false
            break
        } else if (getResult.status == "succeeded") {
            statusCard.innerHTML = `<a href="${getResult.url}">done!</a>`
            submitButton.disabled = false
            break
        }

        await sleep(3)
    }

})
