#!/usr/bin/env node
// Bulk-import tasks through the API (Node 18+).
//   node tools/seed.js [count]
const count = parseInt(process.argv[2]) || 100;
const base = process.env.BASE_URL || "http://localhost:8080";
const token = process.env.BUGHUNT_TOKEN || "dev-token";

async function create(i) {
  const res = await fetch(`${base}/api/tasks`, {
    method: "POST",
    headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
    body: JSON.stringify({ title: `seeded task ${i}`, priority: (i % 5) + 1, tags: ["seed"] }),
  });
  return res.json();
}

async function main() {
  const jobs = [];
  for (let i = 0; i < count; i++) jobs.push(create(i));
  const results = await Promise.all(jobs);
  console.log(`created ${results.length} tasks`);
}

main();
