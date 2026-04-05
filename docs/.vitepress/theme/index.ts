import DefaultTheme from "vitepress/theme";
import type { Theme } from "vitepress";
import { VPTeamMembers } from "vitepress/theme";
import Mermaid from "../components/Mermaid.vue";
import "./custom.css";

export default {
  extends: DefaultTheme,
  enhanceApp({ app }) {
    app.component("Mermaid", Mermaid);
    app.component("VPTeamMembers", VPTeamMembers);
  },
} satisfies Theme;
