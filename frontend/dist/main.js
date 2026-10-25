import { api } from "./api.js";
const state = {
    page: 1,
    limit: 20,
    status: "",
    query: "",
    tasks: [],
};
const $ = (sel) => document.querySelector(sel);
const NEXT_STATUS = { todo: "doing", doing: "done", done: "todo" };
async function loadTasks() {
    const params = new URLSearchParams({ page: String(state.page), limit: String(state.limit) });
    if (state.status)
        params.set("status", state.status);
    if (state.query)
        params.set("q", state.query);
    const res = await api.listTasks(params);
    state.tasks = res.items;
    renderTasks();
}
function renderTasks() {
    const list = $("#task-list");
    list.innerHTML = state.tasks
        .map((t) => `
    <li class="task ${t.status}" data-id="${t.id}">
      <div>
        <strong>${t.title}</strong>
        <span class="tags">${(t.tags || []).join(", ")}</span>
        <div class="desc">${t.description || ""}</div>
      </div>
      <div class="actions">
        <span class="prio">P${t.priority}</span>
        <button data-act="advance">${t.status} &rarr; ${NEXT_STATUS[t.status]}</button>
        <button data-act="delete">Delete</button>
      </div>
    </li>`)
        .join("");
    $("#page-label").textContent = `page ${state.page}`;
    list.addEventListener("click", onListClick);
}
async function onListClick(ev) {
    const btn = ev.target.closest("button");
    if (!btn)
        return;
    const id = Number(btn.closest("li").dataset.id);
    const task = state.tasks.find((t) => t.id === id);
    if (btn.dataset.act === "delete") {
        await api.deleteTask(id);
    }
    else if (btn.dataset.act === "advance") {
        await api.updateTask({ ...task, status: NEXT_STATUS[task.status] });
    }
    await refreshAll();
}
async function refreshAll() {
    await Promise.all([loadTasks(), loadStats()]);
}
async function loadStats() {
    const s = await api.stats();
    $("#stats").innerHTML = `
    <b>${s.total}</b> tasks &middot;
    todo ${s.byStatus.todo ?? 0} / doing ${s.byStatus.doing ?? 0} / done ${s.byStatus.done ?? 0}
    &middot; overdue <b>${s.overdue}</b> &middot; avg priority ${s.avgPriority.toFixed(1)}
    <br/>top tags: ${s.topTags.map((t) => `${t.tag} (${t.count})`).join(", ")}`;
}
async function loadWordstats() {
    const out = $("#wordstats");
    const words = await api.wordstats();
    out.textContent = words.map((w) => `${w.word}: ${w.count}`).join("  |  ");
}
async function loadRuntime() {
    const r = await api.runtime();
    $("#runtime").textContent = `goroutines=${r.goroutines} heap=${r.heapAllocMB.toFixed(1)}MB tasks=${r.tasks}`;
}
function wire() {
    $("#search").addEventListener("input", (ev) => {
        state.query = ev.target.value;
        state.page = 1;
        loadTasks();
    });
    $("#status-filter").addEventListener("change", (ev) => {
        state.status = ev.target.value;
        state.page = 1;
        loadTasks();
    });
    $("#prev").addEventListener("click", () => {
        state.page--;
        loadTasks();
    });
    $("#next").addEventListener("click", () => {
        state.page++;
        loadTasks();
    });
    $("#add-form").addEventListener("submit", async (ev) => {
        ev.preventDefault();
        const form = ev.target;
        const data = new FormData(form);
        await api.createTask({
            title: String(data.get("title")),
            description: String(data.get("description")),
            priority: Number(data.get("priority")),
            tags: String(data.get("tags"))
                .split(",")
                .map((t) => t.trim())
                .filter(Boolean),
        });
        form.reset();
        await refreshAll();
    });
    $("#btn-wordstats").addEventListener("click", loadWordstats);
    $("#btn-runtime").addEventListener("click", loadRuntime);
}
wire();
refreshAll().catch((err) => console.error("initial load failed", err));
//# sourceMappingURL=main.js.map