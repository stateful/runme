import {
    createPromiseClient,
    createConnectTransport,
} from '@bufbuild/connect-web'
import { DeleteSessionRequest, PostSessionRequest, ListSessionsRequest, ExecuteRequest } from "@buf/stateful_runme.bufbuild_es/runme/kernel/v1/kernel_pb.js"
import { KernelService } from "@buf/stateful_runme.bufbuild_connect-web/runme/kernel/v1/kernel_connectweb.js"
import { Terminal } from "xterm";

const client = createPromiseClient(
    KernelService,
    createConnectTransport({
        baseUrl: 'http://localhost:8080',
    })
)

const sessionIdElem = document.getElementById("sessionID") as HTMLPreElement
const commandElem = document.getElementById("command") as HTMLTextAreaElement
const exitCodeElem = document.getElementById("exitCode") as HTMLPreElement
const sessionsElem = document.getElementById("sessions") as HTMLUListElement

export function setActiveSession(id: string) {
    sessionIdElem.textContent = id
}

function prompt(term: Terminal) {
    term.write('\r\n$ ')
}

const term = new Terminal()
term.open(document.getElementById("terminal") as HTMLDivElement)
term.onData(e => {
    console.log("onData", e.codePointAt(0))

    switch (e) {
        case '\r': // Enter
            term.writeln("")
            break
        case '\u007F': // Backspace (DEL)
            term.write('\b \b')
            break
        case '\u0003': // Ctrl+C
            term.write('^C')
            break
        default:
            term.write(e)
    }
});

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

async function execute() {
    const sessionID = sessionIdElem.textContent
    if (sessionID === null || sessionID === "") {
        return
    }

    const command = commandElem.value
    if (command === "") {
        return
    }

    term.clear()
    exitCodeElem.textContent = ""

    const req = new ExecuteRequest({
        sessionId: sessionID,
        command: command,
    })

    for await (const resp of client.executeStream(req)) {
        console.log("execute response", resp)

        term.write(resp.data)

        if (resp.exitCode != undefined) {
            exitCodeElem.textContent = resp.exitCode?.toString()
        }
    }
}

export function handleExecute() {
    execute();
}
