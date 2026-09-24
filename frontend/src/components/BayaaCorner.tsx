import { useState } from 'react';
import { Sparkles } from 'lucide-react';

const jokes = [
  'Баяаа: “Татаж дууссан уу?” — Дөнгөж холбоосоо хуулж байна шүү дээ 😂',
  'Баяаа 32 холболт хараад: “32 найз зэрэг татаж өгч байгаа юм уу?” 🤝',
  'ZIP нь 99%… Баяаагийн тэвчээр 1% 🫠',
  'Баяаа кофе аваад ирэх хооронд файл нь түрүүлээд ирчихлээ ☕',
];

export function BayaaCorner() {
  const [index, setIndex] = useState(0);
  const [collapsed, setCollapsed] = useState(false);
  return <section id="bayaa-corner" className="bayaa-corner" aria-label="Найзуудын хөгжилтэй булан">
    <div className="bayaa-heading"><span className="bayaa-avatar" aria-hidden="true">😎</span><div><strong>Суга Баяаа</strong><small>Найзуудын булан · зүгээр л наргиа</small></div><button aria-expanded={!collapsed} aria-controls="bayaa-joke" onClick={() => setCollapsed(v => !v)}>{collapsed ? 'Харуулах' : 'Хураах'}</button></div>
    <div id="bayaa-joke" hidden={collapsed}><p aria-live="polite">{jokes[index]}</p><button className="bayaa-next" onClick={() => setIndex(i => (i + 1) % jokes.length)}><Sparkles size={14}/> Баяаа дахиад юу гэв?</button></div>
  </section>;
}
