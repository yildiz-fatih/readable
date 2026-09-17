const API_BASE_URL = "http://localhost:8080"
const POLL_INTERVAL_SECONDS = 3
const HTTP_STATUS_ACCEPTED = 202

const JOB_STATUS_PENDING = "pending"
const JOB_STATUS_FAILED = "failed"
const JOB_STATUS_SUCCEEDED = "succeeded"

const STATUS = { WORKING: "working", ERROR: "error", SUCCESS: "success" }

const form = document.getElementById("readable-form")
const submitButton = document.getElementById("submit-button")
const statusCard = document.getElementById("status-card")

form.addEventListener("submit", handleFormSubmit)

async function handleFormSubmit(event) {
    event.preventDefault()
    submitButton.disabled = true

    try {
        // form values
        const url = document.getElementById("url").value
        const format = document.querySelector('input[name="format"]:checked').value

        // enqueue the readable job
        let postResponse
        try {
            postResponse = await postJSON(`${API_BASE_URL}/readables`, { "url": url, "format": format })
        } catch (error) {
            setStatus(STATUS.ERROR)
            return
        }

        if (postResponse.status != HTTP_STATUS_ACCEPTED) {
            setStatus(STATUS.ERROR)
            return
        }

        setStatus(STATUS.WORKING)

        const jobID = postResponse.body.id

        // poll until job finishes
        while (true) {
            /*
                * TODO: Retry up to n times in a row, STATUS.ERROR if all attempts fail
            */
            let getResponse
            try {
                getResponse = await fetch(`${API_BASE_URL}/readables/${jobID}`)
            } catch (error) {
                setStatus(STATUS.ERROR)
                break
            }

            if (!getResponse.ok) {
                setStatus(STATUS.ERROR)
                break
            }

            const job = await getResponse.json()

            if (job.status == JOB_STATUS_FAILED) {
                setStatus(STATUS.ERROR)
                break
            }

            if (job.status == JOB_STATUS_SUCCEEDED) {
                setStatus(STATUS.SUCCESS, job.url)
                break
            }

            // job.status == JOB_STATUS_PENDING
            setStatus(STATUS.WORKING)
            await sleep(POLL_INTERVAL_SECONDS)
        }
    } finally {
        submitButton.disabled = false
    }
}

function setStatus(state, downloadUrl) {
    switch (state) {
        case STATUS.WORKING:
            statusCard.textContent = "working the magic..."
            statusCard.className = "status-working"
            break

        case STATUS.ERROR:
            statusCard.textContent = "Houston, we've had a problem here..."
            statusCard.className = "status-error"
            break

        case STATUS.SUCCESS:
            statusCard.innerHTML = `<a href="${downloadUrl}" class="download-button" download target="_blank">⬇ Download</a>`
            statusCard.className = ""
            break

        default:
            throw new Error(`Unknown status state: ${state}`)
    }
}

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
