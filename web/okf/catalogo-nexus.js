(() => {
 const $=s=>document.querySelector(s), $$=s=>[...document.querySelectorAll(s)];
 const esc=s=>String(s??'').replace(/[&<>"]/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;'}[c]));
 const icons=()=>window.lucide&&window.lucide.createIcons();
 const toast=(message,error=false)=>{const t=$('#toast');t.textContent=message;t.classList.toggle('is-error',error);t.hidden=false;setTimeout(()=>t.hidden=true,error?7000:3500)};
 const st={items:[],account:'',edit:null,draft:null};
 async function api(path,opts={}) { const r=await fetch(path,{headers:opts.body?{'Content-Type':'application/json'}:{},...opts}); const d=r.headers.get('content-type')?.includes('json')?await r.json():await r.text(); if(!r.ok)throw Error(d?.error||r.statusText); return d; }
 async function reload(){try{const d=await api('/api/catalogo-nexus/recursos');st.items=d.recursos||[];st.account=d.account||'';draw()}catch(e){$('#recursosCatalogoNexus').innerHTML='<div class="cat-vazio">'+esc(e.message)+'</div>'}}
 function draw(){
  const query=($('#filtroCatalogoNexus').value||'').trim().toLowerCase();
  const list=st.items.filter(r=>!query||[r.name,r.titulo,r.description,r.kind,r.visibility].join(' ').toLowerCase().includes(query));
  $('#catalogoNexusResumo').textContent=st.items.length+' recursos · '+st.account;
  if(!list.length){
   $('#recursosCatalogoNexus').innerHTML='<div class="cat-vazio">'+(st.items.length?'nada com esse filtro':'nenhum recurso escolhido ainda')+'</div>';
   return;
  }
  const labels={leitura:['Leitura','plug'],escrita:['Escrita','pen-line'],conhecimento:['Conhecimento','book-open'],memoria:['Memória','brain'],acao:['Ação','zap'],agente:['Agente','bot'],form:['Form','file-input']};
  const groups={};
  list.forEach(r=>{
   const key=({knowledge:'conhecimento',memory:'memoria',action:'acao',activity:'acao',agent:'agente',form:'form'})[r.kind]||(r.write?'escrita':'leitura');
   (groups[key]||(groups[key]=[])).push(r);
  });
  let closed={};
  try{closed=JSON.parse(localStorage.getItem('okf.catalogo-nexus.fechados')||'{}')}catch(e){}
  const card=r=>{
   const params=(r.params||[]).map(p=>esc(p.name)+(p.type?':'+esc(p.type):'')+(p.required?'*':'')).join(', ')||'sem parâmetros';
   return `<div class="cat-card"><span class="res-icon"><i data-lucide="plug"></i></span><div class="ct"><strong>${esc(r.titulo||r.name)}</strong>${r.titulo?`<span class="mono">${esc(r.name)}</span>`:''}<span>${esc(r.description||'')}</span><span class="assin">(${params})</span><span class="chips"><span class="tag tag-kind">${esc(r.kind)}</span><span class="tag ${r.visibility==='public'?'tag-public':'tag-private'}">${esc(r.visibility)}</span>${r.write?'<span class="tag tag-write">write</span>':''}</span></div><span style="display:flex;gap:6px"><button class="vote lapis" data-edit="${esc(r.name)}" title="editar"><i data-lucide="pencil"></i></button><button class="vote no" data-remove="${esc(r.name)}" title="remover"><i data-lucide="trash-2"></i></button></span></div>`;
  };
  $('#recursosCatalogoNexus').innerHTML=Object.keys(labels).filter(k=>groups[k]).map(k=>{
   const cards=groups[k].map(card).join('');
   return `<details class="cat-details" data-group="${k}" ${closed[k]&&!query?'':'open'}><summary class="cat-group"><i data-lucide="chevron-right" class="i-sm chev"></i><i data-lucide="${labels[k][1]}"></i>${labels[k][0]}<span class="tag tag-kind">${groups[k].length}</span></summary><div class="cat-body">${cards}</div></details>`;
  }).join('');
  $$('[data-edit]').forEach(b=>b.onclick=()=>showForm(st.items.find(r=>r.name===b.dataset.edit),true));
  $$('[data-remove]').forEach(b=>b.onclick=()=>remove(b.dataset.remove));
  $$('details[data-group]').forEach(d=>d.addEventListener('toggle',()=>{
   closed[d.dataset.group]=!d.open;
   try{localStorage.setItem('okf.catalogo-nexus.fechados',JSON.stringify(closed))}catch(e){}
  }));
  icons();
 }
 const headerRow=(name='',value='')=>'<tr><td><input class="field-input mono" data-hname value="'+esc(name)+'"></td><td><input class="field-input mono" data-hvalue value="'+esc(value)+'"></td><td><button type="button" class="vote no" data-row-delete><i data-lucide="x"></i></button></td></tr>';
 const saidaRow=(item={})=>'<tr><td><input class="field-input mono" data-sname value="'+esc(item.name||'')+'"></td><td><input class="field-input mono" data-spath value="'+esc(item.path||'')+'"></td><td><select class="field-select" data-stype>'+['','string','number','integer','boolean','date','array','object'].map(type=>'<option value="'+type+'" '+(item.type===type?'selected':'')+'>'+(type||'—')+'</option>').join('')+'</select></td><td><input class="field-input" data-sdesc value="'+esc(item.description||'')+'"></td><td><button type="button" class="vote no" data-row-delete><i data-lucide="x"></i></button></td></tr>';
 const paramRow=p=>'<tr><td><input class="field-input mono" data-pname value="'+esc(p?.name||'')+'"></td><td><select class="field-select" data-ptype>'+['','string','number','integer','boolean','date','array','object'].map(t=>'<option value="'+t+'" '+((p?.type||'')===t?'selected':'')+'>'+(t||'—')+'</option>').join('')+'</select></td><td><input type="checkbox" data-preq '+(p?.required?'checked':'')+'></td><td><input class="field-input" data-pdesc value="'+esc(p?.description||'')+'"></td><td><button type="button" class="vote no" data-pdel><i data-lucide="x"></i></button></td></tr>';
 function showSearch(){st.edit=null;st.draft=null;const f=$('#formCatalogoNexus');f.reset();f.dataset.original='';f.hidden=true;$('#catalogoNexusBusca').hidden=false;$('#catalogoNexusBuscaStatus').textContent='';$('#catalogoNexusResultados').innerHTML='';$('#catalogoNexusQuery').value='';$('#catalogoNexusEvidence').textContent='';$('#catalogoNexusDefinition').textContent='';$('#catalogoNexusFormStatus').textContent='';$('#drawerCatalogoNexusTitulo').textContent='Novo recurso';$('#btnVoltarCatalogoNexus').hidden=false;$('#btnRemoverCatalogoNexus').hidden=true;$('#drawerCatalogoNexus').hidden=false;$('#catalogoNexusQuery').focus()}
 function showForm(resource,editing){
  st.edit=editing?resource:null;const f=$('#formCatalogoNexus');f.reset();f.dataset.original=editing?resource.name:'';
  const x=resource||st.draft||{};$('#drawerCatalogoNexusTitulo').textContent=editing?(resource.titulo||resource.name):'Revisar recurso';
  $('#catalogoNexusBusca').hidden=true;f.hidden=false;$('#btnVoltarCatalogoNexus').hidden=!!editing;$('#btnRemoverCatalogoNexus').hidden=!editing;
  f.name.value=x.name||'';f.titulo.value=x.titulo||'';f.kind.value=x.kind||'api';f.visibility.value=x.visibility||'';f.write.checked=!!x.write;f.description.value=x.description||'';
  f.metodo.value=x.conexao?.metodo||'';f.url.value=x.conexao?.url||'';f.corpo.value=x.conexao?.corpo||'';f.output.value=x.output||'';
  f.amostra.value=x.amostra===undefined||x.amostra===null?'':JSON.stringify(x.amostra,null,2);
  const headers=x.conexao?.headers||{};$('#tabCatalogoNexusHeaders tbody').innerHTML=Object.keys(headers).map(key=>headerRow(key,headers[key])).join('');
  $('#tabCatalogoNexusParams tbody').innerHTML=(x.params||[]).map(paramRow).join('');$('#tabCatalogoNexusSaida tbody').innerHTML=(x.saida||[]).map(saidaRow).join('');
  $('#catalogoNexusEvidence').textContent=x.evidence||'Detalhes do catálogo não armazenados para este recurso.';
  $('#catalogoNexusDefinition').textContent=x.definition||'Definição OpenAPI não armazenada para este recurso.';
  $('#drawerCatalogoNexus').hidden=false;icons();f.name.focus();
 }
 async function search(){const q=$('#catalogoNexusQuery').value.trim();if(!q){$('#catalogoNexusBuscaStatus').textContent='Informe o que deseja encontrar.';return}const b=$('#btnBuscarCatalogoNexus');b.disabled=true;$('#catalogoNexusBuscaStatus').textContent='Pesquisando no catálogo…';$('#catalogoNexusResultados').innerHTML='';try{const d=await api('/api/catalogo-nexus/search',{method:'POST',body:JSON.stringify({q})}),ops=d.resultados||[];$('#catalogoNexusBuscaStatus').textContent=ops.length?ops.length+' operações encontradas. Selecione uma para revisar.':'Nenhuma operação encontrada.';$('#catalogoNexusResultados').innerHTML=ops.map((o,i)=>'<button type="button" class="cat-card" data-op="'+i+'"><span class="res-icon"><i data-lucide="plug"></i></span><span class="ct"><strong>'+esc(o.title||o.operationId)+'</strong><span>'+esc(o.method)+' '+esc(o.path)+'</span><span>'+esc(o.serviceName||'')+'</span><span class="mono">'+esc(o.operationId)+'</span></span></button>').join('');$$('[data-op]').forEach(x=>x.onclick=()=>detail(ops[Number(x.dataset.op)]));icons()}catch(e){$('#catalogoNexusBuscaStatus').textContent=e.message;toast(e.message,true)}finally{b.disabled=false}}
 async function detail(o){$('#catalogoNexusBuscaStatus').textContent='Lendo detalhes da operação…';try{const d=await api('/api/catalogo-nexus/detail',{method:'POST',body:JSON.stringify({serviceId:o.serviceId,operationId:o.operationId})});const id=String(o.operationId||'operacao').toLowerCase().replace(/[^a-z0-9_.-]+/g,'-').replace(/^[.-]+|[.-]+$/g,'')||'operacao';st.draft={name:'catalogo.'+id,titulo:d.title||o.title||'',kind:'api',visibility:'',write:['POST','PUT','PATCH','DELETE'].includes(String(d.method||'').toUpperCase()),description:d.description||'',params:d.params||[],evidence:d.evidence||'',conexao:{metodo:d.method||'',url:d.url||''},definition:d.definition||''};showForm(st.draft,false)}catch(e){$('#catalogoNexusBuscaStatus').textContent=e.message;toast(e.message,true)}}
 function payload(f){
  const resource={...(st.edit||st.draft||{})},headers={};
  $$('#tabCatalogoNexusHeaders tbody tr').forEach(row=>{const name=row.querySelector('[data-hname]').value.trim();if(name)headers[name]=row.querySelector('[data-hvalue]').value.trim()});
  resource.name=f.name.value.trim();resource.titulo=f.titulo.value.trim();resource.kind=f.kind.value;resource.visibility=f.visibility.value;resource.write=f.write.checked;resource.description=f.description.value.trim();
  resource.params=$$('#tabCatalogoNexusParams tbody tr').map(row=>({name:row.querySelector('[data-pname]').value.trim(),type:row.querySelector('[data-ptype]').value,required:row.querySelector('[data-preq]').checked,description:row.querySelector('[data-pdesc]').value.trim()})).filter(x=>x.name);
  resource.saida=$$('#tabCatalogoNexusSaida tbody tr').map(row=>({name:row.querySelector('[data-sname]').value.trim(),path:row.querySelector('[data-spath]').value.trim(),type:row.querySelector('[data-stype]').value,description:row.querySelector('[data-sdesc]').value.trim()})).filter(x=>x.name);
  resource.output=f.output.value.trim();if(f.amostra.value.trim())resource.amostra=JSON.parse(f.amostra.value);else delete resource.amostra;
  resource.conexao={metodo:f.metodo.value.trim(),url:f.url.value.trim(),headers,corpo:f.corpo.value.trim()};
  if(!resource.conexao.metodo&&!resource.conexao.url&&!Object.keys(headers).length&&!resource.conexao.corpo)resource.conexao=null;
  return{original:f.dataset.original,recurso:resource};
 }
 async function save(e){e.preventDefault();try{const p=payload(e.target);await api('/api/catalogo-nexus/recurso',{method:'PUT',body:JSON.stringify(p)});$('#drawerCatalogoNexus').hidden=true;toast(p.recurso.name+' salvo no catálogo');await reload()}catch(x){$('#catalogoNexusFormStatus').textContent=x.message;toast(x.message,true)}}
 async function remove(name){if(!confirm('Remover o recurso '+name+' do Catálogo Nexus Recursos?'))return;try{await api('/api/catalogo-nexus/recurso?nome='+encodeURIComponent(name),{method:'DELETE'});$('#drawerCatalogoNexus').hidden=true;toast(name+' removido');await reload()}catch(e){toast(e.message,true)}}
 $('#btnRecarregar').addEventListener('click',()=>setTimeout(reload,0));$('#btnNovoRecursoCatalogo').onclick=showSearch;$('#btnBuscarCatalogoNexus').onclick=search;$('#catalogoNexusQuery').onkeydown=e=>{if(e.key==='Enter'){e.preventDefault();search()}};$('#filtroCatalogoNexus').oninput=draw;$('#btnVoltarCatalogoNexus').onclick=()=>{$('#formCatalogoNexus').hidden=true;$('#catalogoNexusBusca').hidden=false};$('#formCatalogoNexus').onsubmit=save;$('#formCatalogoNexus').onclick=e=>{const b=e.target.closest('[data-pdel],[data-row-delete]');if(b)b.closest('tr').remove()};$('#btnAddCatalogoNexusParam').onclick=()=>{$('#tabCatalogoNexusParams tbody').insertAdjacentHTML('beforeend',paramRow({}));icons()};$('#btnAddCatalogoNexusHeader').onclick=()=>{$('#tabCatalogoNexusHeaders tbody').insertAdjacentHTML('beforeend',headerRow());icons()};$('#btnAddCatalogoNexusSaida').onclick=()=>{$('#tabCatalogoNexusSaida tbody').insertAdjacentHTML('beforeend',saidaRow());icons()};$('#btnRemoverCatalogoNexus').onclick=()=>st.edit&&remove(st.edit.name);document.addEventListener('keydown',e=>{if(e.key==='Escape')$('#drawerCatalogoNexus').hidden=true});
 const section=()=>{if(location.hash==='#/catalogo-nexus')reload()};$$('a[data-secao="catalogo-nexus"]').forEach(a=>a.addEventListener('click',()=>setTimeout(reload,0)));window.addEventListener('hashchange',section);section();
})();
