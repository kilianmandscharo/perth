<script lang="ts">
    import { CircleCheck, CircleQuestionMark } from "@lucide/svelte";

    let { value, hidden }: { value: string | null; hidden: boolean } = $props();
</script>

<div class="vote">
    {#if hidden}
        {#if value}
            <CircleCheck />
        {:else}
            <CircleQuestionMark />
        {/if}
    {:else}
        <div>{value}</div>
    {/if}
</div>
