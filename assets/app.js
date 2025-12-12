(function(){
  const data = window.MICRO_API_DATA || [];
  const list = document.getElementById('apis-list');
  const year = document.getElementById('year');
  if(year) year.textContent = new Date().getFullYear();

  function makeRow(api){
    const row = document.createElement('div');
    row.className = 'api-row';
    const info = document.createElement('div');
    info.className = 'api-info';
    const meta = document.createElement('div');
    meta.className = 'api-meta';
    const name = document.createElement('div'); name.className = 'api-name'; name.textContent = api.name;
    const desc = document.createElement('div'); desc.className = 'api-desc'; desc.textContent = api.description;
    meta.appendChild(name); meta.appendChild(desc);
    info.appendChild(meta);

    const usage = document.createElement('div'); usage.className = 'usage';
    const barwrap = document.createElement('div'); barwrap.className='bar';
    const bar = document.createElement('i'); bar.style.width = '0%';
    barwrap.appendChild(bar);
    usage.appendChild(barwrap);
    const meta2 = document.createElement('div'); meta2.className='meta';
    const left = document.createElement('div'); left.textContent = `${api.price_cents}¢ / call`; left.className='inline-small';
    const right = document.createElement('div'); right.textContent = `${api.usage_pct}% usage`; right.className='inline-small';
    meta2.appendChild(left); meta2.appendChild(right);
    usage.appendChild(meta2);

    const actions = document.createElement('div'); actions.className='actions';
    const btnPreview = document.createElement('button'); btnPreview.className='btn'; btnPreview.textContent='Preview';
    btnPreview.onclick = ()=> showPreview(api);
    const btnBuy = document.createElement('a'); btnBuy.className='btn primary'; btnBuy.href = '#buy'; btnBuy.textContent='Sell';
    actions.appendChild(btnPreview); actions.appendChild(btnBuy);

    row.appendChild(info);
    row.appendChild(usage);
    row.appendChild(actions);

    // animate bar after render
    setTimeout(()=>{ bar.style.width = Math.max(2, Math.min(100, api.usage_pct)) + '%'; }, 140);

    return row;
  }

  function renderList(){
    list.innerHTML = '';
    data.forEach(api => list.appendChild(makeRow(api)));
  }

  function showPreview(api){
    const modal = document.createElement('div');
    modal.style = 'position:fixed;left:0;top:0;right:0;bottom:0;background:rgba(0,0,0,.4);display:flex;align-items:center;justify-content:center;z-index:9999';
    const card = document.createElement('div'); card.className='api-preview'; card.style.width='680px';
    card.innerHTML = `<h3>${api.name} <small style="color:#64748b"> — ${api.id}</small></h3>
      <p class="inline-small">${api.description}</p>
      <pre style="background:#f6fbff;padding:12px;border-radius:6px;margin-top:8px">POST /v1/${api.id}
{
  "data": { "text": "Tu texto aquí" }
}</pre>
      <p style="margin-top:8px"><strong>Precio guía:</strong> ${api.price_cents}¢ / call · <span class="inline-small">${api.usage_pct}% uso demo</span></p>
      <div style="display:flex;gap:8px;justify-content:flex-end;margin-top:12px">
        <button id="close" class="btn">Cerrar</button>
        <button id="demo" class="btn primary">Agregar a landing</button>
      </div>`;
    modal.appendChild(card);
    document.body.appendChild(modal);
    document.getElementById('close').onclick = ()=> document.body.removeChild(modal);
    document.getElementById('demo').onclick = ()=>{
      document.body.removeChild(modal);
      addToLanding(api);
    };
  }

  function addToLanding(api){
    const res = document.getElementById('order-result');
    res.innerHTML = `<div><strong>Se agregó:</strong> ${api.name} — listo para mostrar en la landing de venta.</div>`;
    // Store selection (simple)
    const chosen = JSON.parse(localStorage.getItem('chosen_apis')||'[]');
    if(!chosen.find(x=>x.id===api.id)) chosen.push(api);
    localStorage.setItem('chosen_apis', JSON.stringify(chosen));
  }

  function orderNow(e){
    e.preventDefault();
    const plan = document.getElementById('order-plan').value;
    const email = document.getElementById('order-email').value;
    const chosen = JSON.parse(localStorage.getItem('chosen_apis')||'[]');
    const out = document.getElementById('order-result');
    if(!email){ out.textContent = 'Email requerido'; return false; }
    // Generate a simple landing page content as downloadable blob
    const payload = { plan, email, apis: chosen, generated_at: new Date().toISOString() };
    const blob = new Blob([generateLandingHtml(payload)], {type:'text/html'});
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a'); a.href=url; a.download = `landing_${Date.now()}.html`; a.textContent='Descargar landing';
    out.innerHTML = '';
    out.appendChild(a);
    return false;
  }

  function generateLandingHtml(payload){
    const apisHtml = (payload.apis||[]).map(a=>`<li><strong>${a.name}</strong> — ${a.description} — ${a.price_cents}¢/call</li>`).join('');
    return `<!doctype html><html><head><meta charset="utf-8"><title>Landing - ${payload.email}</title></head><body><h1>Oferta ${payload.plan}</h1><p>Contacto: ${payload.email}</p><ul>${apisHtml}</ul><p>Generated: ${payload.generated_at}</p></body></html>`;
  }

  renderList();
})();

