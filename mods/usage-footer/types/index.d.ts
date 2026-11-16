export type Label = string | null

declare module 'claude-code' {
  interface PluginState {
    'usage-footer': { label: Label }
  }
}
