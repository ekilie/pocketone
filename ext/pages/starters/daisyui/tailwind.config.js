/** @type {import('tailwindcss').Config} */
export default {
  content: ['./pb_pages/**/*.{ets,md}'],
  theme: {
    extend: {},
  },
  daisyui: {
    themes: ['cupcake'],
  },
  plugins: [require('@tailwindcss/typography'), require('daisyui')],
}
