import '../public/theme.css';
import '../public/base.css';
import '../public/components.css';
import '../public/utils.css';
import '../public/responsive.css';
import App from './App.svelte';

const app = new App({ target: document.getElementById('app')! });

export default app;
