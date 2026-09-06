import '@testing-library/jest-dom/vitest';
import { cleanup } from '@testing-library/react';
import { afterEach, vi } from 'vitest';

Object.defineProperty(window, 'matchMedia', { writable: true, value: vi.fn().mockImplementation(() => ({ matches: false })) });
afterEach(() => { cleanup(); localStorage.clear(); delete document.documentElement.dataset.theme; });
