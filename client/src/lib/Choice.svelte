<script lang="ts">
    let { value, hidden }: { value: string | null; hidden: boolean } = $props();
</script>

<div>{hidden ? value ? "✅" : "❓": value}</div>
