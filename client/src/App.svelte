<script lang="ts">
    import Choices from "./lib/Choices.svelte";
    import Choice from "./lib/Choice.svelte";
    import type { AppState, Category, Payload, User } from "./types";

    const values = [
        "1",
        "2",
        "4",
        "6",
        "8",
        "12",
        "16",
        "20",
        "24",
        "28",
        "32",
        "40",
        "?",
    ];

    let ws: WebSocket | null = $state(null);

    let loading = $state(true);
    let showNewUserInput = $state(true);
    let password = $state("");
    let name = $state("");

    let invalidPassword = $state(false);
    let nameDuplicate = $state(false);

    let notifications: {
        level: "success" | "error";
        msg: string;
        id: string;
    }[] = $state([]);

    let appState: AppState = $state([]);

    let showVotes = $derived(
        appState
            .filter((user) => user.connected)
            .every((user) => user.bets.every((bet) => !!bet)),
    );

    /*
    actions:
    - login, send name, password, receive uuid
    - init, send uuid, receive state
    - vote, send uuid, receive update
    - pong, send uuid, receive nothing
    - restart, send uuid, receive update
    */

    $effect(() => {
        console.log("connecting socket...");

        const socket = new WebSocket("ws://localhost:8080");

        socket.onopen = () => {
            const uuid = getKey();
            if (uuid) {
                socket.send(
                    JSON.stringify({ action: "init", data: { id: uuid } }),
                );
                showNewUserInput = false;
            }
            loading = false;
        };

        socket.onclose = () => {
            console.log("closed");
        };

        socket.onmessage = (event) => {
            try {
                const payload: Payload = JSON.parse(event.data);
                switch (payload.action) {
                    case "login":
                        console.log("login successful", payload.uuid);
                        setKey(payload.uuid);
                        showNewUserInput = false;
                        socket.send(
                            JSON.stringify({
                                action: "init",
                                data: { id: payload.uuid },
                            }),
                        );
                        break;
                    case "newUser":
                        console.log("new user", payload.name);
                        appState.push({
                            name: payload.name,
                            connected: true,
                            notAnswering: false,
                            ping: 999,
                            bets: [null, null, null],
                        });
                        break;
                    case "reconnect":
                        console.log("reconnect", payload.name);
                        updateUserByName(payload.name, "connected", true);
                        break;
                    case "init":
                        console.log("init", payload.data);
                        appState = payload.data;
                        break;
                    case "vote":
                        console.log("vote", payload.data);
                        const item = appState.find(
                            (item) => item.name === payload.data.name,
                        );
                        if (item) {
                            item.bets[payload.data.category] =
                                payload.data.value;
                        }
                        break;
                    case "disconnect":
                        console.log("disconnect", payload.name);
                        updateUserByName(payload.name, "connected", false);
                        break;
                    case "invalidPassword":
                        invalidPassword = true;
                        setTimeout(() => {
                            invalidPassword = false;
                        }, 3000);
                        notify("Invalid Password", "error");
                        break;
                    case "nameDuplicate":
                        nameDuplicate = true;
                        setTimeout(() => {
                            nameDuplicate = false;
                        }, 3000);
                        notify("Name already exists", "error");
                        break;
                    case "ping":
                        console.log("ping");
                        socket.send(
                            JSON.stringify({
                                action: "pong",
                                data: { id: getKey() },
                            }),
                        );
                        break;
                    case "pingUpdate":
                        console.log("pingUpdate");
                        updateUserByName(payload.name, "ping", payload.ping);
                        break;
                    case "reset":
                        console.log("reset");
                        for (const user of appState) {
                            for (let i = 0; i < user.bets.length; i++) {
                                user.bets[i] = null;
                            }
                        }
                        break;
                    case "unknownId":
                        console.log("unknown id");
                        showNewUserInput = true;
                }
            } catch (err) {
                console.error(event.data);
                console.error(err);
            }
        };

        ws = socket;

        return () => {
            socket.close();
        };
    });

    function updateUserByName<T extends keyof User>(
        name: string,
        field: T,
        value: User[T],
    ) {
        const user = appState.find((u) => u.name === name);
        if (user) user[field] = value;
    }

    function login() {
        ws?.send(JSON.stringify({ action: "login", data: { name, password } }));
    }

    function vote(value: string, category: Category) {
        let categoryNumber = -1;
        switch (category) {
            case "Optimistic":
                categoryNumber = 0;
                break;
            case "Realistic":
                categoryNumber = 1;
                break;
            case "Pessimistic":
                categoryNumber = 2;
                break;
        }
        ws?.send(
            JSON.stringify({
                action: "vote",
                data: {
                    value,
                    category: categoryNumber,
                    id: getKey(),
                },
            }),
        );
    }

    function reset() {
        ws?.send(JSON.stringify({ action: "reset", data: { id: getKey() } }));
    }

    function getKey() {
        return localStorage.getItem("uuid");
    }

    function setKey(key: string) {
        return localStorage.setItem("uuid", key);
    }

    function notify(msg: string, level: "error" | "success") {
        const id = self.crypto.randomUUID();
        notifications.push({
            id,
            msg,
            level,
        });
        setTimeout(() => {
            notifications = notifications.filter((n) => n.id !== id);
        }, 3000);
    }
</script>

{#if loading}
    <div>Loading...</div>
{:else if showNewUserInput}
    <div id="landing-page">
        <div id="welcome">
            <div class="above">Welcome to</div>
            <div class="below">Perth</div>
        </div>
        <div id="login">
            <div>
                <label for="name-input">Your name</label>
                <input
                    id="name-input"
                    class={nameDuplicate ? "error" : ""}
                    bind:value={name}
                />
            </div>
            <div>
                <label for="password-input">The server password</label>
                <input
                    id="password-input"
                    class={invalidPassword ? "error" : ""}
                    bind:value={password}
                    type="password"
                />
            </div>
            <button
                onclick={login}
                disabled={password.length === 0 || name.length === 0}
                >Submit</button
            >
        </div>
    </div>
{:else}
    <div id="app">
        <button onclick={reset}>Reset</button>
        <div id="overview">
            {#each appState as user (user.name)}
                {#if user.connected}
                    <div class="user-container">
                        <div class="user-connection"></div>
                        <div class="user-ping">{user.ping}ms</div>
                        <div class="user-name">{user.name}</div>
                        <div class="user-bets">
                            <Choice value={user.bets[0]} hidden={!showVotes} />
                            <Choice value={user.bets[1]} hidden={!showVotes} />
                            <Choice value={user.bets[2]} hidden={!showVotes} />
                        </div>
                    </div>
                {/if}
            {/each}
        </div>
        <Choices name="Optimistic" {values} onClick={vote} />
        <Choices name="Realistic" {values} onClick={vote} />
        <Choices name="Pessimistic" {values} onClick={vote} />
    </div>
{/if}

{#if notifications.length > 0}
    <div id="notifications">
        {#each notifications as notification (notification.id)}
            <div id="notification" class={notification.level}>
                {notification.msg}
            </div>
        {/each}
    </div>
{/if}
