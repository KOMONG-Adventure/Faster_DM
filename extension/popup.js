const input=document.querySelector('#url'),status=document.querySelector('#status'),send=document.querySelector('#send');
chrome.tabs.query({active:true,currentWindow:true}).then(tabs=>{if(/^https?:/.test(tabs[0]?.url||''))input.value=tabs[0].url;}).catch(()=>{});
chrome.storage.local.get('lastMessage').then(s=>{status.textContent=s.lastMessage||'';});
chrome.action.setBadgeText({text:''});
document.querySelector('#form').addEventListener('submit',async event=>{
  event.preventDefault();send.disabled=true;status.textContent='Дамжуулж байна…';
  try{const result=await chrome.runtime.sendMessage({type:'send',url:input.value.trim()});status.textContent=result?.message||'Хариу ирсэнгүй.';}catch{status.textContent='Өргөтгөлийг дахин нээгээд оролдоно уу.';}finally{send.disabled=false;}
});
document.querySelector('#recent').addEventListener('click',async()=>{
  try{
    if(!await chrome.permissions.request({permissions:['downloads']}))return;
    const items=await chrome.downloads.search({limit:10,orderBy:['-startTime']});const list=document.querySelector('#downloads');list.replaceChildren();
    for(const item of items){const url=item.finalUrl||item.url;if(!/^https?:/.test(url))continue;const b=document.createElement('button');b.type='button';b.textContent=item.filename.split(/[\\/]/).pop()||url;b.addEventListener('click',()=>{input.value=url;status.textContent='Холбоос сонголоо. Апп руу дамжуулах товчийг дарна уу.';});list.append(b);}
    if(!list.children.length)status.textContent='Дамжуулах боломжтой HTTP/HTTPS таталт алга.';
  }catch{status.textContent='Браузерын таталтын жагсаалтыг уншиж чадсангүй.';}
});
