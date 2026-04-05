<script setup lang="ts">
import { ref, onMounted, watch, nextTick } from 'vue'
import { useData } from 'vitepress'

const props = defineProps<{
  source: string
}>()

const { isDark } = useData()
const rendered = ref('')
const diagramId = `mermaid-${Math.random().toString(36).substr(2, 9)}`

async function renderDiagram() {
  try {
    const mermaid = (await import('mermaid')).default
    mermaid.initialize({
      startOnLoad: false,
      theme: isDark.value ? 'dark' : 'default',
      securityLevel: 'loose',
      flowchart: {
        useMaxWidth: true,
        htmlLabels: true,
        curve: 'basis'
      }
    })
    const { svg } = await mermaid.render(diagramId, props.source)
    rendered.value = svg
  } catch (e) {
    console.error('Mermaid render error:', e)
    rendered.value = '<p style="color: red;">Diagram failed to render</p>'
  }
}

onMounted(async () => {
  await nextTick()
  await renderDiagram()
})

watch(isDark, async () => {
  await renderDiagram()
})
</script>

<template>
  <div class="mermaid-wrapper" v-html="rendered"></div>
</template>

<style scoped>
.mermaid-wrapper {
  margin: 1.5rem auto;
  padding: 1rem;
  background: var(--vp-c-bg-soft);
  border-radius: 8px;
  overflow-x: auto;
  text-align: center;
}
</style>