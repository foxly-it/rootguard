export type MessageValues = Record<string, string | number>;

export interface LocaleDefinition {
  code: string;
  label: string;
  messages: Record<string, string>;
}
