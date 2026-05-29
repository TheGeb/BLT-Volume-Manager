import '../public/theme.css';
import '../public/base.css';
import '../public/components.css';
import '../public/utils.css';
import '../public/responsive.css';
import { mount } from 'svelte';
import App from './App.svelte';

const app = mount(App, { target: document.getElementById('app')! });

export default app;
