// frontend/src/ir/schema.ts
import { z } from 'zod'

// ── Upstream ──────────────────────────────────────────

const UpstreamSchema = z.object({
  name:     z.string(),
  provider: z.enum(['kong', 'k8s-service', 'url']).default('kong'),
  url:      z.string().optional(),
})

// ── 節點類型 ──────────────────────────────────────────

const HttpCallNodeSchema = z.object({
  id:   z.string(),
  type: z.literal('http-call'),
  config: z.object({
    upstream: UpstreamSchema,
    path:     z.string(),
    method:   z.enum(['GET', 'POST', 'PUT', 'DELETE', 'PATCH']).default('GET'),
    timeout:  z.number().int().positive().default(3000),
    headers:  z.record(z.string()).optional(),
    body:     z.string().optional(),
    retry: z.object({
      maxAttempts: z.number().int().default(1),
      backoff:     z.enum(['fixed', 'exponential']).default('fixed'),
      delay:       z.number().default(1000),
    }).optional(),
    fallback: z.object({
      strategy: z.enum(['default-value', 'skip', 'error']).default('error'),
      value:    z.any().optional(),
    }).optional(),
  }),
  outputVar: z.string(),
})

const ConditionNodeSchema = z.object({
  id:   z.string(),
  type: z.literal('condition'),
  config: z.object({
    expression: z.string(),
  }),
})

const TransformNodeSchema = z.object({
  id:   z.string(),
  type: z.literal('transform'),
  config: z.object({
    engine:     z.enum(['jsonata', 'jmespath']).default('jsonata'),
    expression: z.string(),
  }),
  outputVar: z.string(),
})

const ForkNodeSchema = z.object({
  id:   z.string(),
  type: z.literal('fork'),
  config: z.object({
    strategy: z.enum(['all', 'race', 'allSettled']).default('all'),
    timeout:  z.number().optional(),
  }),
})

const JoinNodeSchema = z.object({
  id:   z.string(),
  type: z.literal('join'),
  config: z.object({
    strategy:   z.enum(['merge', 'array', 'custom']).default('merge'),
    expression: z.string().optional(),
  }),
  outputVar: z.string(),
})

const SubFlowNodeSchema = z.object({
  id:   z.string(),
  type: z.literal('sub-flow'),
  config: z.object({
    flowId:   z.string(),
    inputMap: z.record(z.string()),
  }),
  outputVar: z.string(),
})

const ResponseNodeSchema = z.object({
  id:   z.string(),
  type: z.literal('response'),
  config: z.object({
    statusCode: z.number().int().default(200),
    body:       z.string(),
    headers:    z.record(z.string()).optional(),
  }),
})

// ── Union + Edge + 頂層 ──────────────────────────────

export const IRNodeSchema = z.discriminatedUnion('type', [
  HttpCallNodeSchema,
  ConditionNodeSchema,
  TransformNodeSchema,
  ForkNodeSchema,
  JoinNodeSchema,
  SubFlowNodeSchema,
  ResponseNodeSchema,
])

export const IREdgeSchema = z.object({
  source:       z.string(),
  target:       z.string(),
  sourceHandle: z.string().nullable().optional(),
})

export const GatewayIRSchema = z.object({
  version: z.literal('1.0'),
  id:      z.string(),
  name:    z.string(),
  trigger: z.object({
    method: z.enum(['GET', 'POST', 'PUT', 'DELETE', 'PATCH', 'ANY']),
    path:   z.string(),
  }),
  nodes:    z.array(IRNodeSchema).min(1),
  edges:    z.array(IREdgeSchema),
  executionHints: z.object({
    parallelGroups: z.array(z.array(z.string())).optional(),
  }).optional(),
  metadata: z.object({
    createdAt: z.string().datetime(),
    updatedAt: z.string().datetime(),
    author:    z.string().optional(),
  }).optional(),
})

// ── 匯出型別 ─────────────────────────────────────────

export type GatewayIR = z.infer<typeof GatewayIRSchema>
export type IRNode    = z.infer<typeof IRNodeSchema>
export type IREdge    = z.infer<typeof IREdgeSchema>
export type Upstream  = z.infer<typeof UpstreamSchema>

// 個別節點型別（方便 switch narrowing）
export type HttpCallNode  = z.infer<typeof HttpCallNodeSchema>
export type ConditionNode = z.infer<typeof ConditionNodeSchema>
export type TransformNode = z.infer<typeof TransformNodeSchema>
export type ForkNode      = z.infer<typeof ForkNodeSchema>
export type JoinNode      = z.infer<typeof JoinNodeSchema>
export type SubFlowNode   = z.infer<typeof SubFlowNodeSchema>
export type ResponseNode  = z.infer<typeof ResponseNodeSchema>

// 節點類型常數
export const NODE_TYPES = [
  'http-call', 'condition', 'transform',
  'fork', 'join', 'sub-flow', 'response',
] as const

export type NodeType = (typeof NODE_TYPES)[number]
