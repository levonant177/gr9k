/** @type {import('tailwindcss').Config} */
export default {
  content: ['./src/**/*.{html,js,svelte,ts}'],
  theme: {
    extend: {
      colors: {
        // Light Telegram design system
        tg: {
          bg: '#F0F2F5',
          card: '#FFFFFF',
          accent: '#3390EC',
          accentHover: '#2B7CD3',
          text: '#0E1621',
          textSecondary: '#707579',
          border: 'rgba(0, 0, 0, 0.08)',
          success: '#31B545',
          warning: '#E5A00D',
          danger: '#E53935'
        }
      },
      fontFamily: {
        sans: ['Inter', 'system-ui', 'sans-serif'],
        mono: ['JetBrains Mono', 'ui-monospace', 'monospace']
      },
      boxShadow: {
        glass: '0 4px 24px rgba(0, 0, 0, 0.06)',
        card: '0 1px 3px rgba(0, 0, 0, 0.08), 0 1px 2px rgba(0, 0, 0, 0.06)'
      },
      backdropBlur: {
        glass: '12px'
      }
    }
  },
  plugins: []
};
