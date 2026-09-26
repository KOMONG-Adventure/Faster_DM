const HOST = 'com.komong.fasterdm';
chrome.runtime.onInstalled.addListener(() => {
  chrome.contextMenus.create({ id: 'fasterdm-send', title: 'Faster DM рүү дамжуулах', contexts: ['link', 'video', 'audio'] });
});
function validURL(value) {
  try { const u = new URL(value); return ['http:', 'https:'].includes(u.protocol) && !u.username && !u.password && value.length <= 16384; } catch { return false; }
}
async function send(url) {
  if (!validURL(url)) return { ok: false, message: 'Шууд HTTP/HTTPS холбоос шаардлагатай. blob: болон нэвтрэх эрхтэй холбоос дэмжихгүй.' };
  try { return await chrome.runtime.sendNativeMessage(HOST, { url }); }
  catch { return { ok: false, message: 'Faster DM installer-ийг суулгана уу. Browser bridge блоклогдсон бол Windows хамгаалалтын мэдэгдлийг шалгана уу.' }; }
}
chrome.contextMenus.onClicked.addListener(async info => {
  if (info.menuItemId !== 'fasterdm-send') return;
  const reply = await send(info.linkUrl || info.srcUrl);
  await chrome.storage.local.set({ lastMessage: reply.message });
  await chrome.action.setBadgeText({ text: reply.ok ? '✓' : '!' });
});
chrome.runtime.onMessage.addListener((message, sender, respond) => {
  if (sender.id !== chrome.runtime.id || message.type !== 'send') return;
  send(message.url).then(respond, () => respond({ ok: false, message: 'Дамжуулж чадсангүй.' }));
  return true;
});
