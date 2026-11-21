import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { registerFonts } from './theme/fonts';
import './theme/tokens.css';
import './theme/base.css';
import { Boot } from './Boot';

registerFonts();

const root = document.getElementById('root');
if (root) {
  createRoot(root).render(
    <StrictMode>
      <Boot />
    </StrictMode>,
  );
}
