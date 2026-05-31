import '../public/theme.css';
import '../public/base.css';
import '../public/components.css';
import '../public/utils.css';
import '../public/responsive.css';
import { mount } from 'svelte';
import App from './App.svelte';

const target = document.getElementById('app');
if (!target) throw new Error('root element #app not found');
const app = mount(App, { target });

export default app;
