import {
    createPromiseClient,
    createConnectTransport,
} from '@bufbuild/connect-web'
import { DeleteSessionRequest, PostSessionRequest, ListSessionsRequest, ExecuteRequest, InputRequest, OutputRequest } from "@buf/stateful_runme.bufbuild_es/runme/kernel/v1/kernel_pb.js"
import { KernelService } from "@buf/stateful_runme.bufbuild_connect-web/runme/kernel/v1/kernel_connectweb.js"
import { Terminal } from "xterm";

const client = createPromiseClient(
    KernelService,
    createConnectTransport({
        baseUrl: 'http://localhost:8080',
    })
)

const sessionIdElem = document.getElementById("sessionID") as HTMLPreElement
const sessionsElem = document.getElementById("sessions") as HTMLUListElement

export function setActiveSession(id: string) {
    sessionIdElem.textContent = id

    subscribeOutput()
}

function prompt(term: Terminal) {
    term.write("bash-3.2$ ")
}

const term = new Terminal()
term.open(document.getElementById("terminal") as HTMLDivElement)

let buffer = ""
let lastCommand = ""

term.onData(async e => {
    console.log("onData", e.codePointAt(0))

    switch (e) {
        case '\r': // Enter
            const command = buffer + "\n"

            lastCommand = buffer
            buffer = ""

            console.log("sending input", command)

            const resp = await client.input(new InputRequest({
                sessionId: sessionIdElem.textContent || undefined,
                data: new TextEncoder().encode(command),
            }))
            console.log("input response", resp)

            break
        case '\u007F': // Backspace (DEL)
            if (buffer.length > 0) {
                term.write('\b \b');
                if (buffer.length > 0) {
                    buffer = buffer.substr(0, buffer.length - 1);
                }
            }
            break
        case '\u0003': // Ctrl+C
            term.write('^C')
            break
        default:
            if (e >= String.fromCharCode(0x20) && e <= String.fromCharCode(0x7E) || e >= '\u00a0') {
                buffer += e
                term.write(e)
            }
    }
});

prompt(term)

async function subscribeOutput() {
    const req = new OutputRequest({
        sessionId: sessionIdElem.textContent || undefined,
    })

    for await (const resp of client.output(req)) {
        const data = new TextDecoder().decode(resp.data)
        console.log("output response", data)

        if (data == lastCommand) {
            continue
        }

        term.write(data)
    }

    console.log("finished reading output")
}

async function createSession() {
    // Prompt can be auto-detected but it will work only in bash 4.4+.
    let req = new PostSessionRequest({
        command: "/bin/bash",
        prompt: "bash-3.2$",
    })

    const resp = await client.postSession(req)
    console.log("postSession response", resp)
    sessionIdElem.textContent = resp.session?.id || ""
}

export function handleCreateSession() {
    createSession()
}

async function deleteSession() {
    const sessionID = sessionIdElem.textContent
    if (sessionID === null || sessionID === "") {
        return
    }

    let req = new DeleteSessionRequest({
        sessionId: sessionID,
    })

    const resp = await client.deleteSession(req)
    console.log("deleteSession response", resp)
    sessionIdElem.textContent = ''
}

export function handleDeleteSession() {
    deleteSession()
}

async function listSessions() {
    let content = "";

    const resp = await client.listSessions(new ListSessionsRequest())
    for (const s of resp.sessions) {
        content += `<li>${s.id} <button onclick="runme.setActiveSession('${s.id}')">Activate</button></li>`
    }

    sessionsElem.innerHTML += content
}

export function handleListSessions() {
    listSessions()
}
