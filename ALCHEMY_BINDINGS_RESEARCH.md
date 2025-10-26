# How Alchemy Handles Cloudflare Worker Bindings

## Overview

The Alchemy project handles Cloudflare Worker bindings through a sophisticated multi-layered approach that:

1. **Retrieves binding information** from the Cloudflare API
2. **Defines comprehensive TypeScript types** for all binding types
3. **Converts between formats** (Alchemy internal format ↔ Cloudflare API format ↔ wrangler.json format)
4. **Manages local and remote bindings** for development and production environments

## Key Finding: Getting Binding Information from Cloudflare API

### The Core Function: `getWorkerSettings()`

**File:** `/home/user/alchemy_repo/alchemy/src/cloudflare/worker-metadata.ts` (lines 708-726)

```typescript
export async function getWorkerSettings(
  api: CloudflareApi,
  workerName: string,
): Promise<WorkerSettings | undefined> {
  // Fetch the bindings for a worker by calling the Cloudflare API endpoint:
  // GET /accounts/:account_id/workers/scripts/:script_name/settings
  // See: https://developers.cloudflare.com/api/resources/workers/subresources/scripts/subresources/script_and_version_settings/methods/get/
  return await extractCloudflareResult<WorkerSettings>(
    `get worker settings for ${workerName}`,
    api.get(
      `/accounts/${api.accountId}/workers/scripts/${workerName}/settings`,
    ),
  ).catch((error) => {
    if (error.status === 404) {
      return undefined;
    }
    throw error;
  });
}
```

**API Endpoint:** `GET /accounts/:account_id/workers/scripts/:script_name/settings`

**What it returns:**
```typescript
interface WorkerSettings {
  bindings: WorkerBindingSpec[];
  compatibility_date: string;
  compatibility_flags: string[];
  migrations: SingleStepMigration | MultiStepMigration;
  tags: string[];
  [key: string]: any;
}
```

### Critical Detail: Binding Metadata Encoding

Alchemy uses **Worker tags** to store metadata about bindings that isn't directly exposed by the API. See lines 255-278 of worker-metadata.ts:

```typescript
// we use Cloudflare Worker tags to store a mapping between Alchemy's stable identifier and the binding name
// e.g.
// {
//   BINDING_NAME: DurableObjectNamespace("stable-id")
// }
// will be stored as alchemy:do:stable-id:BINDING_NAME

const bindingNameToStableId = Object.fromEntries(
  oldTags?.flatMap((tag: string) => {
    // alchemy:do:{stableId}:{bindingName}
    if (tag.startsWith("alchemy:do:")) {
      const [, , stableId, bindingName] = tag.split(":");
      return [[bindingName, stableId]];
    }
    return [];
  }) ?? [],
);
```

**Why this is needed:** Cloudflare's API doesn't return stable IDs for Durable Objects, so Alchemy encodes them in tags to track them across deployments.

## 1. Binding Type Definitions

### Location: `/home/user/alchemy_repo/alchemy/src/cloudflare/bindings.ts`

Defines all 25+ binding types as TypeScript interfaces. Examples:

```typescript
/**
 * KV Namespace binding type
 */
export interface WorkerBindingKVNamespace {
  /** The name of the binding */
  name: string;
  /** Type identifier for KV Namespace binding */
  type: "kv_namespace";
  /** KV Namespace ID */
  namespace_id: string;
}

/**
 * R2 Bucket binding type
 */
export interface WorkerBindingR2Bucket {
  name: string;
  type: "r2_bucket";
  bucket_name: string;
  jurisdiction?: R2BucketJurisdiction;
}

/**
 * D1 database binding type
 */
export interface WorkerBindingD1 {
  name: string;
  type: "d1";
  id: string;
}

/**
 * Durable Object Namespace binding type
 */
export interface WorkerBindingDurableObjectNamespace {
  stableId?: string;  // Internal stable ID
  name: string;
  type: "durable_object_namespace";
  class_name: string;
  script_name?: string;
  environment?: string;
  namespace_id?: string;  // Encoded in tags
}
```

## 2. Converting to Cloudflare API Format

### Location: `/home/user/alchemy_repo/alchemy/src/cloudflare/worker-metadata.ts` (lines 223-619)

Function `prepareWorkerMetadata()` converts Alchemy bindings to Cloudflare API format:

```typescript
// Convert bindings to the format expected by the API
for (const [bindingName, binding] of Object.entries(bindings)) {
  if (typeof binding === "string") {
    meta.bindings.push({
      type: "plain_text",
      name: bindingName,
      text: binding,
    });
  } else if (binding.type === "kv_namespace") {
    meta.bindings.push({
      type: "kv_namespace",
      name: bindingName,
      namespace_id:
        "namespaceId" in binding ? binding.namespaceId : binding.id,
    });
  } else if (binding.type === "r2_bucket") {
    meta.bindings.push({
      type: "r2_bucket",
      name: bindingName,
      bucket_name: binding.name,
      jurisdiction:
        binding.jurisdiction === "default" ? undefined : binding.jurisdiction,
    });
  } else if (binding.type === "d1") {
    meta.bindings.push({
      type: "d1",
      name: bindingName,
      id: binding.id,
    });
  } else if (binding.type === "durable_object_namespace") {
    meta.bindings.push({
      type: "durable_object_namespace",
      name: bindingName,
      class_name: binding.className,
      script_name:
        binding.scriptName === props.workerName
          ? undefined
          : binding.scriptName,
      environment: binding.environment,
      namespace_id: binding.namespaceId,
    });
  }
  // ... more binding types
}
```

## 3. Converting to wrangler.json Format

### Location: `/home/user/alchemy_repo/alchemy/src/cloudflare/wrangler.json.ts` (lines 548-945)

Function `processBindings()` converts bindings to wrangler.json format:

```typescript
function processBindings(
  spec: WranglerJsonSpec,
  bindings: Bindings,
  // ...
): void {
  const kvNamespaces: {
    binding: string;
    id: string;
    preview_id: string;
    remote?: boolean;
  }[] = [];

  const d1Databases: {
    binding: string;
    database_id: string;
    database_name: string;
    migrations_dir?: string;
    preview_database_id: string;
    remote?: boolean;
  }[] = [];

  // ... process each binding type

  for (const [bindingName, binding] of Object.entries(bindings)) {
    if (binding.type === "kv_namespace") {
      const id =
        "dev" in binding && !binding.dev?.remote && local
          ? binding.dev.id      // Use local ID for dev mode
          : binding.namespaceId; // Use production ID
      kvNamespaces.push({
        binding: bindingName,
        id: id,
        preview_id: id,
        remote: binding.dev?.remote,
      });
    } else if (binding.type === "r2_bucket") {
      const name =
        "dev" in binding && !binding.dev?.remote && local
          ? binding.dev.id
          : binding.name;
      r2Buckets.push({
        binding: bindingName,
        bucket_name: name,
        preview_bucket_name: name,
        jurisdiction: binding.jurisdiction,
      });
    } else if (binding.type === "d1") {
      const id =
        "dev" in binding && !binding.dev?.remote && local
          ? binding.dev.id
          : binding.id;
      d1Databases.push({
        binding: bindingName,
        database_id: id,
        database_name: binding.name,
        migrations_dir: binding.migrationsDir,
        preview_database_id: id,
        remote: binding.dev?.remote,
      });
    }
  }

  // Add to spec
  if (kvNamespaces.length > 0) {
    spec.kv_namespaces = kvNamespaces;
  }
  if (d1Databases.length > 0) {
    spec.d1_databases = d1Databases;
  }
  // ... etc
}
```

## 4. Remote Binding Proxy for Development

### Files:
- `/home/user/alchemy_repo/alchemy/workers/remote-binding-proxy.ts`
- `/home/user/alchemy_repo/alchemy/src/cloudflare/miniflare/build-worker-options.ts`

When bindings are marked as "remote" or for certain types that can't be emulated locally, Alchemy creates a proxy worker that routes binding requests:

```typescript
// From remote-binding-proxy.ts - lines 35-64

function getExposedJSRPCBinding(request: Request, env: Env) {
  const url = new URL(request.url);
  const bindingName = url.searchParams.get("MF-Binding");
  if (!bindingName) {
    throw new BindingNotFoundError();
  }

  const targetBinding = env[bindingName];
  if (!targetBinding) {
    throw new BindingNotFoundError(bindingName);
  }

  // Special handling for different binding types
  if (targetBinding.constructor.name === "SendEmail") {
    return {
      async send(e: ForwardableEmailMessage) {
        const message = new EmailMessage(e.from, e.to, e["EmailMessage::raw"]);
        return (targetBinding as SendEmail).send(message);
      },
    };
  }

  if (url.searchParams.has("MF-Dispatch-Namespace-Options")) {
    const { name, args, options } = JSON.parse(
      url.searchParams.get("MF-Dispatch-Namespace-Options")!,
    );
    return (targetBinding as DispatchNamespace).get(name, args, options);
  }

  return targetBinding;
}
```

## 5. Local vs Remote Binding Strategy

### Location: `/home/user/alchemy_repo/alchemy/src/cloudflare/miniflare/build-worker-options.ts` (lines 152-200)

```typescript
export const buildWorkerOptions = async (
  input: MiniflareWorkerInput,
): Promise<{
  watch: (signal: AbortSignal) => AsyncGenerator<miniflare.WorkerOptions>;
  remoteProxy: HTTPServer | undefined;
}> => {
  const remoteBindings: RemoteBinding[] = [];
  
  for (const [key, binding] of Object.entries(input.bindings ?? {})) {
    switch (binding.type) {
      // Bindings that can ONLY be remote (AI, Browser, Vectorize, etc.)
      case "ai": {
        remoteBindings.push({
          type: "ai",
          name: key,
          raw: true,
        });
        break;
      }

      // Bindings that can be local OR remote depending on config
      case "kv_namespace": {
        if (isRemoteBinding(binding)) {
          remoteBindings.push({
            type: "kv_namespace",
            name: key,
            namespace_id: binding.namespaceId,
            raw: true,
          });
        } else {
          (options.kvNamespaces ??= {})[key] = {
            id: binding.dev.id,
            remoteProxyConnectionString: remoteProxy.connectionString,
          };
        }
        break;
      }

      case "d1": {
        if (isRemoteBinding(binding)) {
          remoteBindings.push({
            type: "d1",
            name: key,
            id: binding.id,
            raw: true,
          });
        } else {
          (options.d1Databases ??= {})[key] = binding.dev.id;
        }
        break;
      }

      case "r2_bucket": {
        if (isRemoteBinding(binding)) {
          remoteBindings.push({
            type: "r2_bucket",
            name: key,
            bucket_name: binding.name,
            raw: true,
          });
        } else {
          (options.r2Buckets ??= {})[key] = {
            id: binding.name,
            remoteProxyConnectionString: remoteProxy.connectionString,
          };
        }
        break;
      }
    }
  }
};
```

## 6. Complete List of Supported Binding Types

From `bindings.ts`, Alchemy supports:

1. **AI** (`type: "ai"`) - AI/ML models
2. **Analytics Engine** (`type: "analytics_engine"`) - Time-series analytics
3. **Assets** (`type: "assets"`) - Static files/Workers Assets
4. **Browser Rendering** (`type: "browser"`) - Puppeteer rendering
5. **D1 Database** (`type: "d1"`) - SQLite databases
6. **Dispatch Namespace** (`type: "dispatch_namespace"`) - Workflows dispatch
7. **Durable Object Namespace** (`type: "durable_object_namespace"`) - Stateful objects
8. **Hyperdrive** (`type: "hyperdrive"`) - Database connection pooling
9. **Images** (`type: "images"`) - Image optimization
10. **KV Namespace** (`type: "kv_namespace"`) - Key-value store
11. **MTLS Certificate** (`type: "mtls_certificate"`) - Client certificates
12. **Pipeline** (`type: "pipelines"`) - Scheduled jobs
13. **Plain Text** (`type: "plain_text"`) - Environment variables
14. **Queue** (`type: "queue"`) - Message queues
15. **Rate Limit** (`type: "ratelimit"`) - Rate limiting
16. **R2 Bucket** (`type: "r2_bucket"`) - Object storage
17. **Secret Key** (`type: "secret_key"`) - Cryptographic keys
18. **Secret Text** (`type: "secret_text"`) - Encrypted secrets
19. **Secrets Store Secret** (`type: "secrets_store_secret"`) - Secret management
20. **Service** (`type: "service"`) - Cross-worker communication
21. **Static Content** (`type: "static_content"`) - Static files
22. **Tail Consumer** (`type: "tail_consumer"`) - Log streaming
23. **Vectorize** (`type: "vectorize"`) - Vector databases
24. **Version Metadata** (`type: "version_metadata"`) - Deployment info
25. **WASM Module** (`type: "wasm_module"`) - WebAssembly modules
26. **Worker Loader** (`type: "worker_loader"`) - Worker imports
27. **Workflow** (`type: "workflow"`) - Durable workflows

## Key Insights for cf-delete-worker

1. **Binding Information Source:**
   - Primary: Cloudflare API endpoint `GET /accounts/:account_id/workers/scripts/:script_name/settings`
   - Secondary: Worker tags contain metadata (especially for Durable Objects)

2. **Binding Identification:**
   - Each binding has a `name` (the binding variable name in the worker)
   - Each binding has a `type` (one of the 27 types above)
   - Additional properties vary by type (e.g., `namespace_id` for KV, `bucket_name` for R2, `class_name` for DO)

3. **Development vs Production:**
   - Bindings can have `dev` configuration for local development
   - Remote bindings route through a proxy worker in dev mode
   - Alchemy uses `preview_id` for local versions of KV/D1

4. **Stable Identification:**
   - For Durable Objects, stable IDs are encoded in worker tags as `alchemy:do:{stableId}:{bindingName}`
   - This allows tracking DO class migrations across deployments

5. **Migration Tracking:**
   - Migration tags are used: `alchemy:migration-tag:{version}`
   - Used to handle class name changes and deletions in Durable Objects

## API Calls Summary

| Operation | Endpoint | Method | Purpose |
|-----------|----------|--------|---------|
| Get bindings | `/accounts/:account_id/workers/scripts/:script_name/settings` | GET | Retrieve all binding definitions |
| Deploy with bindings | `/accounts/:account_id/workers/scripts/:script_name` | PUT | Update worker with new bindings |
| Get script metadata | `/accounts/:account_id/workers/scripts/:script_name` | GET | Retrieve worker metadata |

