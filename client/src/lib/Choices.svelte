<script lang="ts">
    import type { Category } from "../types";

    let {
        name,
        values,
        onClick,
    }: {
        name: Category;
        values: string[];
        onClick: (value: string, category: Category) => void;
    } = $props();
</script>

<div class="choice-block">
    <div>{name}</div>
    <div class="choices">
        {#each values as value}
            <button onclick={() => onClick(value, name)}>{value}</button>
        {/each}
    </div>
</div>
