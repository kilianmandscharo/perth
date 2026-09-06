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
      action: "init";
      data: AppState;
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

export type AppState = {
  name: string;
  connected: boolean;
  notAnswering: boolean;
  ping: number;
  bets: [string | null, string | null, string | null];
}[];

export type Category = "Optimistic" | "Realistic" | "Pessimistic";
