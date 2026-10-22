const TOKEN = "dev-token"; // must match BUGHUNT_TOKEN if you start the server with one
async function request(path, init = {}) {
    const res = await fetch(path, {
        ...init,
        headers: {
            "Content-Type": "application/json",
            Authorization: `Bearer ${TOKEN}`,
            ...(init.headers || {}),
        },
    });
    return res.json();
}
export const api = {
    listTasks: (params) => request(`/api/tasks?${params}`),
    createTask: (t) => request("/api/tasks", { method: "POST", body: JSON.stringify(t) }),
    updateTask: (t) => request(`/api/tasks/${t.id}`, { method: "PUT", body: JSON.stringify(t) }),
    deleteTask: (id) => request(`/api/tasks/${id}`, { method: "DELETE" }),
    stats: () => request("/api/stats"),
    wordstats: () => request("/api/wordstats"),
    runtime: () => request("/api/debug/runtime"),
};
//# sourceMappingURL=api.js.map