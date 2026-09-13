<script lang="ts">
    import Choices from "./lib/Choices.svelte";
    import Vote from "./lib/Vote.svelte";
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
            .every((user) => user.votes.every((vote) => !!vote)),
    );

    let results: {
        opti: number;
        real: number;
        pess: number;
        pert: number;
        sd: number;
    } | null = $derived(
        (() => {
            const opti = getAverage(0);
            const real = getAverage(1);
            const pess = getAverage(2);
            const pert = getPert(opti, real, pess);
            const sd = getSd(opti, pess);
            return showVotes
                ? {
                      opti,
                      real,
                      pess,
                      pert,
                      sd,
                  }
                : null;
        })(),
    );

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

                        const newUser: User = {
                            name: payload.name,
                            connected: true,
                            ping: 999,
                            votes: [null, null, null],
                        };

                        const index = appState.findIndex(
                            (u) => u.name === payload.name,
                        );

                        if (index > -1) {
                            appState[index] = newUser;
                        } else {
                            appState.push(newUser);
                        }
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
                        console.log("vote", payload);
                        const item = appState.find(
                            (item) => item.name === payload.name,
                        );
                        if (item) {
                            item.votes[payload.category] = payload.value;
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
                            for (let i = 0; i < user.votes.length; i++) {
                                user.votes[i] = null;
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

    function handleKeyDownOnLanding(e: KeyboardEvent) {
        if (e.key === "Enter") {
            login();
        }
    }

    function getAverage(pos: number) {
        const users = appState.filter((u) => u.connected);
        const nums = users
            .map((u) => {
                const val = u.votes[pos];
                if (val === null || val === "?") return null;
                const num = parseInt(val, 10);
                return num;
            })
            .filter(Boolean) as number[];
        return nums.reduce((acc, num) => acc + num, 0) / nums.length;
    }

    function getPert(opti: number, real: number, pess: number) {
        return (opti + 4 * real + pess) / 6;
    }

    function getSd(opti: number, pess: number) {
        return (pess - opti) / 6;
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
                    onkeydown={handleKeyDownOnLanding}
                />
            </div>
            <div>
                <label for="password-input">The server password</label>
                <input
                    id="password-input"
                    class={invalidPassword ? "error" : ""}
                    bind:value={password}
                    type="password"
                    onkeydown={handleKeyDownOnLanding}
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
                        <div class="info">
                            <div class="connection"></div>
                            <div class="ping">
                                {user.ping === 999 ? "?" : user.ping}ms
                            </div>
                            <div class="name">{user.name}</div>
                        </div>
                        <div class="user-votes">
                            <Vote value={user.votes[0]} hidden={!showVotes} />
                            <Vote value={user.votes[1]} hidden={!showVotes} />
                            <Vote value={user.votes[2]} hidden={!showVotes} />
                        </div>
                    </div>
                {/if}
            {/each}
        </div>
        {#if showVotes}
            <div id="results">
                <div>
                    Opti -> {results?.opti}
                </div>
                <div>
                    Real -> {results?.real}
                </div>
                <div>
                    Pess -> {results?.pess}
                </div>
                <div>
                    PERT -> {results?.pert}
                </div>
                <div>
                    Standard Deviation -> {results?.sd}
                </div>
            </div>
        {/if}
        <Choices
            name="Optimistic"
            {values}
            onClick={vote}
            disabled={showVotes}
        />
        <Choices
            name="Realistic"
            {values}
            onClick={vote}
            disabled={showVotes}
        />
        <Choices
            name="Pessimistic"
            {values}
            onClick={vote}
            disabled={showVotes}
        />
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
