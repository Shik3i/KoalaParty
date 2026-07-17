import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vitest/config';
export default defineConfig({ plugins: [sveltekit()], server: { proxy: { '/api': 'http://localhost:8080' } }, test: { environment: 'jsdom', include: ['src/**/*.test.ts'] } });
