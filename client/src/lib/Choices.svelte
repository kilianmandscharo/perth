<script lang="ts">
    import type { Category } from "../types";

    let {
        name,
        values,
        onClick,
        disabled,
    }: {
        name: Category;
        values: string[];
        onClick: (value: string, category: Category) => void;
        disabled: boolean;
    } = $props();
</script>

<div class="choice-block">
    <div class="choice-type">{name}</div>
    <div class="choices">
        {#each values as value}
            <button {disabled} onclick={() => onClick(value, name)}
                >{value}</button
            >
        {/each}
    </div>
</div>
