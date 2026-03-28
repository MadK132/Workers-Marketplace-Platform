/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{js,jsx}"],
  theme: {
    extend: {
      colors: {
        brand: {
          50: "#eef8ff",
          100: "#d8eeff",
          200: "#b9dfff",
          300: "#8ecbff",
          400: "#5aaeff",
          500: "#338eff",
          600: "#1d6fe9",
          700: "#1958cf",
          800: "#1b4aaa",
          900: "#1c4185",
        },
      },
      boxShadow: {
        panel: "0 20px 50px -30px rgba(15, 23, 42, 0.55)",
      },
    },
  },
  plugins: [],
};
