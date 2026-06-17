import { createClient, type Client } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-node";
import { QQL } from "./gen/qql_pb";

/** Outcome of a single QQL query execution. */
export interface Result {
  ok: boolean;
  operation: string;
  message: string;
  data: unknown | null;
}

/** Gateway and Qdrant connection status. */
export interface HealthStatus {
  version: string;
  qdrantConnected: boolean;
  qdrantStatus: string;
}

function decodeData(bytes: Uint8Array | null): unknown | null {
  if (!bytes || bytes.length === 0) return null;
  try {
    return JSON.parse(new TextDecoder().decode(bytes));
  } catch {
    return null;
  }
}

/** Client for the QQL Gateway (Connect RPC). */
export class QQLClient {
  /** Raw Connect RPC client for direct protobuf usage. */
  public raw: Client<typeof QQL>;

  constructor(
    private url: string = "http://localhost:50051",
    private apiKey?: string,
  ) {
    const transport = createConnectTransport({
      baseUrl: url,
      httpVersion: "2",
      interceptors: apiKey
        ? [
            (next) => async (req) => {
              req.header.set("Authorization", `Bearer ${apiKey}`);
              return await next(req);
            },
          ]
        : [],
    });

    this.raw = createClient(QQL, transport);
  }

  /** Execute a single QQL query. */
  async exec(query: string): Promise<Result> {
    const res = await this.raw.exec({ query });
    return {
      ok: res.ok,
      operation: res.operation,
      message: res.message,
      data: decodeData(res.data),
    };
  }

  /** Execute multiple QQL queries in one round-trip. */
  async execBatch(
    queries: string[],
    stopOnError = false,
  ): Promise<Result[]> {
    const res = await this.raw.execBatch({
      queries: queries.map((q) => ({ query: q })),
      stopOnError,
    });
    return res.results.map((r) => ({
      ok: r.ok,
      operation: r.operation,
      message: r.message,
      data: decodeData(r.data),
    }));
  }

  /** Return the execution plan for a query without running it. */
  async explain(query: string): Promise<string> {
    const res = await this.raw.explain({ query });
    return res.plan;
  }

  /** Check gateway and Qdrant connection status. */
  async health(): Promise<HealthStatus> {
    const res = await this.raw.health({});
    return {
      version: res.version,
      qdrantConnected: res.qdrantConnected,
      qdrantStatus: res.qdrantStatus,
    };
  }
}
