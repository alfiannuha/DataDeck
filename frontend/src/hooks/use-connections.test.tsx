import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import {
  createConnection,
  deleteConnection,
  listConnections,
} from "@/lib/api/endpoints";
import type { ConnectionCreateRequest } from "@/types/api";

import {
  useConnections,
  useCreateConnection,
  useDeleteConnection,
} from "./use-connections";

vi.mock("@/lib/api/endpoints", () => ({
  listConnections: vi.fn(),
  createConnection: vi.fn(),
  testConnection: vi.fn(),
  deleteConnection: vi.fn(),
}));

function createWrapper(queryClient: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
  };
}

function newQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
}

const validCreate: ConnectionCreateRequest = {
  name: "PG",
  driver: "postgres",
  host: "127.0.0.1",
  database_name: "app",
  username: "u",
};

describe("connection hooks", () => {
  afterEach(() => {
    vi.clearAllMocks();
  });

  it("fetches connections with the centralized key", async () => {
    vi.mocked(listConnections).mockResolvedValue([
      { id: "c1", name: "PG", driver: "postgres" } as never,
    ]);
    const queryClient = newQueryClient();

    const { result } = renderHook(() => useConnections(), {
      wrapper: createWrapper(queryClient),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toHaveLength(1);
    expect(listConnections).toHaveBeenCalledTimes(1);
  });

  it("invalidates the connections query after create", async () => {
    vi.mocked(createConnection).mockResolvedValue({ id: "c1" } as never);
    const queryClient = newQueryClient();
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const { result } = renderHook(() => useCreateConnection(), {
      wrapper: createWrapper(queryClient),
    });

    await act(async () => {
      await result.current.mutateAsync(validCreate);
    });

    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["connections"] });
  });

  it("invalidates connections and clears schema cache after delete", async () => {
    vi.mocked(deleteConnection).mockResolvedValue({ id: "c1" } as never);
    const queryClient = newQueryClient();
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
    const removeSpy = vi.spyOn(queryClient, "removeQueries");

    const { result } = renderHook(() => useDeleteConnection(), {
      wrapper: createWrapper(queryClient),
    });

    await act(async () => {
      await result.current.mutateAsync("c1");
    });

    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["connections"] });
    expect(removeSpy).toHaveBeenCalledWith({
      queryKey: ["connections", "c1", "schema"],
    });
    expect(removeSpy).toHaveBeenCalledWith({
      queryKey: ["connections", "c1", "databases"],
    });
  });
});
