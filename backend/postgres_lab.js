"use strict";
async function load(path) {
  const el = document.getElementById(path === "overview" ? "overview" : "catalog");
  el.textContent = "Consultando...";
  try {
    const response = await fetch("/api/v1/postgres/lab/" + path, {cache: "no-store", credentials: "same-origin"});
    const data = await response.json();
    el.textContent = JSON.stringify({status: response.status, data}, null, 2);
  } catch (error) {
    el.textContent = "Falha na consulta: " + error.message;
  }
}
document.querySelectorAll("button[data-query]").forEach(button => {
  button.addEventListener("click", () => load(button.dataset.query));
});
load("overview");
