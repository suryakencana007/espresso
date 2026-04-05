<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useData } from 'vitepress'

const props = defineProps<{
  source: string
}>()

const { isDark } = useData()
const rendered = ref('')
const Mermaid = ref<any>(null)

onMounted(async () => {
  try {
    const mermaid = await import('mermaid')
    mermaid.default.initialize({
      startOnLoad: true,
      theme: isDark.value ? 'dark' : 'default',
      securityLevel: 'loose',
      flowchart: {
        useMaxWidth: true,
        htmlLabels: true,
        curve: 'basis'
      },
      sequence: {
        useMaxWidth: true,
        diagramMarginX: 50,
        diagramMarginY: 10,
        actorMargin: 50,
        width: 150,
        height: 65
      },
      class: {
        useMaxWidth: true
      }
    })

    const { svg } = await mermaid.default.render('mermaid-' + Math.random().toString(36).substr(2, 9), props.source)
    rendered.value = svg
  } catch (e) {
    console.error('Mermaid render error:', e)
    rendered.value = '<p>Diagram failed to render</p>'
  }
})

watch(isDark, async (dark) => {
  const mermaid = await import('mermaid')
  mermaid.default.initialize({
    theme: dark ? 'dark' : 'default'
  })
  const { svg } = await mermaid.default.render('mermaid-' + Math.random().toString(36).substr(2, 9), props.source)
  rendered.value = svg
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

.dark .mermaid-wrapper {
  background: var(--vp-c-bg-alt);
}
</style>