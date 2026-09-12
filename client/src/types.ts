export type Payload =
  | {
      action: "login";
      uuid: string;
    }
  | {
      action: "newUser";
      name: string;
    }
  | {
      action: "reconnect";
      name: string;
    }
  | {
      action: "init";
      data: AppState;
    }
  | {
      action: "disconnect";
      name: string;
    }
  | {
      action: "ping";
    }
  | {
      action: "invalidPassword";
    }
  | {
      action: "nameDuplicate";
    }
  | {
      action: "pingUpdate";
      name: string;
      ping: number;
    }
  | {
      action: "reset";
    }
  | {
      action: "vote";
      data: {
        value: string;
        category: number;
        name: string;
      };
    }
  | {
      action: "unknownId";
    };

export type User = {
  name: string;
  connected: boolean;
  notAnswering: boolean;
  ping: number;
  bets: [string | null, string | null, string | null];
};

export type AppState = User[];

export type Category = "Optimistic" | "Realistic" | "Pessimistic";
