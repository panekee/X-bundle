# Getting Started — Micro API Suite (Panel para venta)

Este repositorio contiene un panel estático listo para publicar en **GitHub Pages** y vender como producto digital (landing + demo). Incluye:

- Panel estático (index.html + assets)
- Lista de 10 APIs con barras de uso demo
- Generador de landing simple (descarga HTML)
- Script `deploy/gh-pages-deploy.sh` para publicar en branch `gh-pages`

## Pasos rápidos para publicar en GitHub Pages

1. Crear repo público en GitHub y subir todos los archivos.
2. Configurar `gh-pages` branch (o usar GitHub Pages desde `/docs` o `gh-pages`).
3. Ejecutar (local):
   ```bash
   chmod +x deploy/gh-pages-deploy.sh
   ./deploy/gh-pages-deploy.sh
