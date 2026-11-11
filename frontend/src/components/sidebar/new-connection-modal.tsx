"use client";

import { Database } from "lucide-react";
import { useEffect, useState, type ReactNode } from "react";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { useCreateConnection, useTestConnection, useUpdateConnection } from "@/hooks/use-connections";
import { useOnline } from "@/hooks/use-online";
import { ApiClientError } from "@/lib/api-client";
import {
  CONNECTION_COLORS,
  DEFAULT_CONNECTION_COLOR,
  connectionColor,
} from "@/lib/connection-colors";
import { cn } from "@/lib/utils";
import type {
  ConnectionCreateRequest,
  ConnectionResponse,
  ConnectionTestRequest,
  ConnectionUpdateRequest,
} from "@/types/api";

type Driver = NonNullable<ConnectionCreateRequest["driver"]>;

const DRIVERS: { value: Driver; label: string }[] = [
  { value: "postgres", label: "PostgreSQL" },
  { value: "mysql", label: "MySQL" },
  { value: "sqlite", label: "SQLite (file)" },
];

const SSL_MODES = [
  "disable",
  "allow",
  "prefer",
  "require",
  "verify-ca",
  "verify-full",
] as const;

function defaultPort(driver: Driver): string {
  switch (driver) {
    case "mysql":
      return "3306";
    case "postgres":
      return "5432";
    default:
      return "";
  }
}

interface FormState {
  driver: Driver;
  name: string;
  host: string;
  port: string;
  database: string;
  username: string;
  password: string;
  sslMode: string;
  color: string;
}

const EMPTY_FORM: FormState = {
  driver: "postgres",
  name: "",
  host: "127.0.0.1",
  port: "5432",
  database: "",
  username: "",
  password: "",
  sslMode: "disable",
  color: DEFAULT_CONNECTION_COLOR,
};

/** Prefill the form from an existing profile (edit mode). The password is never
 * returned by the API, so it stays empty and is preserved by the backend. */
function fromConnection(connection: ConnectionResponse): FormState {
  const driver = (connection.driver ?? "postgres") as Driver;
  return {
    driver,
    name: connection.name ?? "",
    host: connection.host ?? "",
    port: connection.port != null ? String(connection.port) : defaultPort(driver),
    database: connection.database_name ?? "",
    username: connection.username ?? "",
    password: "",
    sslMode: connection.ssl_mode ?? "disable",
    color: connectionColor(connection.color),
  };
}

type FieldName = "name" | "host" | "port" | "database" | "username" | "sslMode";
type FieldErrors = Partial<Record<FieldName, string>>;

interface TestState {
  status: "idle" | "testing" | "success" | "failure";
  message?: string;
}

function isFileDriver(driver: Driver): boolean {
  return driver === "sqlite";
}

/** Mirrors backend validation (backend remains authoritative). */
function validate(form: FormState, requireName: boolean): FieldErrors {
  const errors: FieldErrors = {};
  if (requireName && !form.name.trim()) errors.name = "Name is required.";

  const database = form.database.trim();
  if (!database) {
    if (isFileDriver(form.driver)) {
      errors.database = "Database file path is required.";
    } else if (form.driver === "mysql") {
      // MySQL binds to one database; PostgreSQL profiles are server-level and
      // may omit the database (PRF-01).
      errors.database = "Database is required.";
    }
  }
  if (isFileDriver(form.driver)) {
    return errors; // SQLite: no host/port/credentials/SSL.
  }

  if (!form.host.trim()) errors.host = "Host is required.";
  const port = Number(form.port);
  if (!Number.isInteger(port) || port < 1 || port > 65535) {
    errors.port = "Port must be between 1 and 65535.";
  }
  if (!form.username.trim()) errors.username = "Username is required.";
  if (!(SSL_MODES as readonly string[]).includes(form.sslMode)) {
    errors.sslMode = "Select a valid SSL mode.";
  }
  return errors;
}

function toRequest(form: FormState): ConnectionTestRequest {
  if (isFileDriver(form.driver)) {
    return {
      driver: form.driver,
      database_name: form.database.trim(),
      ssl_mode: "disable",
    };
  }
  return {
    driver: form.driver,
    host: form.host.trim(),
    port: Number(form.port),
    database_name: form.database.trim(),
    username: form.username.trim(),
    password: form.password,
    ssl_mode: form.sslMode as ConnectionTestRequest["ssl_mode"],
  };
}

export function NewConnectionModal({
  open,
  onOpenChange,
  onSaved,
  connection,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSaved?: (connection: ConnectionResponse) => void;
  /** When set, the modal edits this profile (PRF-03/B) instead of creating one. */
  connection?: ConnectionResponse;
}) {
  const isEdit = Boolean(connection?.id);
  const [form, setForm] = useState<FormState>(EMPTY_FORM);
  const [errors, setErrors] = useState<FieldErrors>({});
  const [test, setTest] = useState<TestState>({ status: "idle" });

  const createConnection = useCreateConnection();
  const updateConnection = useUpdateConnection();
  const testConnection = useTestConnection();
  const online = useOnline();

  useEffect(() => {
    if (open) {
      setForm(connection ? fromConnection(connection) : EMPTY_FORM);
      setErrors({});
      setTest({ status: "idle" });
    }
  }, [open, connection]);

  function update(field: keyof FormState, value: string) {
    setForm((prev) => ({ ...prev, [field]: value }));
  }

  function selectDriver(driver: Driver) {
    setForm((prev) => ({
      ...prev,
      driver,
      port: defaultPort(driver),
      sslMode: "disable",
    }));
    setErrors({});
    setTest({ status: "idle" });
  }

  async function handleTest() {
    const nextErrors = validate(form, false);
    setErrors(nextErrors);
    if (Object.keys(nextErrors).length > 0) {
      setTest({ status: "idle" });
      return;
    }
    setTest({ status: "testing" });
    try {
      await testConnection.mutateAsync(toRequest(form));
      setTest({ status: "success", message: "Connection successful." });
    } catch (error) {
      setTest({ status: "failure", message: describeError(error) });
    }
  }

  async function handleSave() {
    const nextErrors = validate(form, true);
    setErrors(nextErrors);
    if (Object.keys(nextErrors).length > 0) return;

    try {
      if (isEdit && connection?.id) {
        const body: ConnectionUpdateRequest = {
          ...toRequest(form),
          name: form.name.trim(),
          color: form.color,
        };
        const updated = await updateConnection.mutateAsync({
          connectionId: connection.id,
          body,
        });
        onSaved?.(updated);
      } else {
        const body: ConnectionCreateRequest = {
          ...toRequest(form),
          name: form.name.trim(),
          color: form.color,
        };
        const created = await createConnection.mutateAsync(body);
        onSaved?.(created);
      }
      onOpenChange(false);
    } catch (error) {
      setTest({ status: "failure", message: describeError(error) });
    }
  }

  const fileDriver = isFileDriver(form.driver);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent aria-labelledby="new-connection-title">
        <DialogHeader>
          <DialogTitle id="new-connection-title">
            {isEdit ? "Edit connection" : "New connection"}
          </DialogTitle>
          <DialogDescription>
            Credentials are encrypted on the backend and never returned.
            {isEdit ? " Leave the password empty to keep the saved one." : ""}
          </DialogDescription>
        </DialogHeader>

        <form
          className="grid grid-cols-2 gap-3"
          onSubmit={(event) => {
            event.preventDefault();
            void handleSave();
          }}
        >
          <Field label="Driver" id="conn-driver" className="col-span-2">
            <div className="flex items-center gap-2">
              <Database
                size={15}
                aria-hidden="true"
                className="shrink-0 text-muted-foreground"
              />
              <Select
                id="conn-driver"
                value={form.driver}
                onChange={(e) => selectDriver(e.target.value as Driver)}
              >
                {DRIVERS.map((driver) => (
                  <option key={driver.value} value={driver.value}>
                    {driver.label}
                  </option>
                ))}
              </Select>
            </div>
          </Field>

          <Field label="Name" id="conn-name" error={errors.name} className="col-span-2">
            <Input
              id="conn-name"
              value={form.name}
              onChange={(e) => update("name", e.target.value)}
              aria-invalid={Boolean(errors.name)}
            />
          </Field>

          <Field label="Color" id="conn-color" className="col-span-2">
            <div className="flex flex-wrap items-center gap-1.5">
              <span
                data-testid="conn-color-preview"
                aria-hidden="true"
                className="h-4 w-4 shrink-0 rounded-full border border-border"
                style={{ backgroundColor: form.color }}
              />
              <div role="radiogroup" aria-label="Connection color" className="flex flex-wrap items-center gap-1.5">
                {CONNECTION_COLORS.map((option) => {
                  const selected = form.color.toLowerCase() === option.value;
                  return (
                    <button
                      key={option.value}
                      type="button"
                      role="radio"
                      aria-checked={selected}
                      aria-label={option.name}
                      title={option.name}
                      onClick={() => update("color", option.value)}
                      className={cn(
                        "h-5 w-5 rounded-full border border-border focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-focus-ring)]",
                        selected && "ring-2 ring-[var(--color-focus-ring)]",
                      )}
                      style={{ backgroundColor: option.value }}
                    />
                  );
                })}
              </div>
              <input
                type="color"
                aria-label="Custom color"
                value={form.color}
                onChange={(e) => update("color", e.target.value)}
                className="h-6 w-8 cursor-pointer rounded border border-border bg-transparent p-0.5"
              />
            </div>
          </Field>

          {fileDriver ? (
            <Field
              label="Database file path"
              id="conn-database"
              error={errors.database}
              className="col-span-2"
            >
              <Input
                id="conn-database"
                placeholder="/path/to/database.db"
                value={form.database}
                onChange={(e) => update("database", e.target.value)}
                aria-invalid={Boolean(errors.database)}
              />
            </Field>
          ) : (
            <>
              <Field
                label="Host"
                id="conn-host"
                error={errors.host}
                className="sm:col-span-1"
              >
                <Input
                  id="conn-host"
                  value={form.host}
                  onChange={(e) => update("host", e.target.value)}
                  aria-invalid={Boolean(errors.host)}
                />
              </Field>
              <Field
                label="Port"
                id="conn-port"
                error={errors.port}
                className="sm:col-span-1"
              >
                <Input
                  id="conn-port"
                  inputMode="numeric"
                  value={form.port}
                  onChange={(e) => update("port", e.target.value)}
                  aria-invalid={Boolean(errors.port)}
                />
              </Field>
              <Field
                label={
                  form.driver === "postgres"
                    ? "Database (optional)"
                    : "Database"
                }
                id="conn-database"
                error={errors.database}
                className="col-span-2"
              >
                <Input
                  id="conn-database"
                  placeholder={
                    form.driver === "postgres" ? "Discover after connecting" : ""
                  }
                  value={form.database}
                  onChange={(e) => update("database", e.target.value)}
                  aria-invalid={Boolean(errors.database)}
                />
                {form.driver === "postgres" && !errors.database && (
                  <span className="text-xs text-muted-foreground">
                    Leave empty to connect to the server. DataDeck uses a
                    PostgreSQL bootstrap database internally and lets you choose
                    a database after saving.
                  </span>
                )}
              </Field>
              <Field
                label="Username"
                id="conn-username"
                error={errors.username}
                className="sm:col-span-1"
              >
                <Input
                  id="conn-username"
                  value={form.username}
                  onChange={(e) => update("username", e.target.value)}
                  aria-invalid={Boolean(errors.username)}
                />
              </Field>
              <Field label="Password" id="conn-password" className="sm:col-span-1">
                <Input
                  id="conn-password"
                  type="password"
                  autoComplete="new-password"
                  value={form.password}
                  onChange={(e) => update("password", e.target.value)}
                />
              </Field>
              <Field
                label="SSL mode"
                id="conn-ssl"
                error={errors.sslMode}
                className="col-span-2"
              >
                <Select
                  id="conn-ssl"
                  value={form.sslMode}
                  onChange={(e) => update("sslMode", e.target.value)}
                >
                  {SSL_MODES.map((mode) => (
                    <option key={mode} value={mode}>
                      {mode}
                    </option>
                  ))}
                </Select>
              </Field>
            </>
          )}

          <div className="col-span-2 min-h-5 text-xs" role="status" aria-live="polite">
            {!online && (
              <span className="text-amber-400">
                Backend unavailable — reconnect to test or save a connection.
              </span>
            )}
            {online && test.status === "testing" && (
              <span className="text-muted-foreground">Testing connection…</span>
            )}
            {online && test.status === "success" && (
              <span className="text-emerald-400">{test.message}</span>
            )}
            {online && test.status === "failure" && (
              <span className="text-destructive">{test.message}</span>
            )}
          </div>

          <DialogFooter className="col-span-2">
            <Button
              variant="outline"
              onClick={() => void handleTest()}
              disabled={
                !online || testConnection.isPending || createConnection.isPending || updateConnection.isPending
              }
            >
              Test connection
            </Button>
            <Button
              type="submit"
              disabled={!online || createConnection.isPending || updateConnection.isPending}
            >
              {createConnection.isPending || updateConnection.isPending
                ? "Saving…"
                : isEdit
                  ? "Save changes"
                  : "Save"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function Field({
  label,
  id,
  error,
  className,
  children,
}: {
  label: string;
  id: string;
  error?: string;
  className?: string;
  children: ReactNode;
}) {
  const errorId = `${id}-error`;
  return (
    <div className={className}>
      <Label htmlFor={id} className="mb-1 block">
        {label}
      </Label>
      {children}
      {error && (
        <p id={errorId} className="mt-1 text-xs text-destructive">
          {error}
        </p>
      )}
    </div>
  );
}

/**
 * Produce a safe, credential-free message from a normalized API error.
 */
function describeError(error: unknown): string {
  if (error instanceof ApiClientError) {
    return `${error.message} (${error.code})`;
  }
  return "Connection failed.";
}
