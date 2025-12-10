const DEMO_KEY = "DEMO_KEY_1234";

document.getElementById("get-key").addEventListener("click", () => {
  navigator.clipboard.writeText(DEMO_KEY)
    .then(() => alert("Key demo copiada: " + DEMO_KEY))
    .catch(() => prompt("Copia la key:", DEMO_KEY));
});
