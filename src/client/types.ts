export interface DaptinQueryParameters {
  [key: string]: string | number | boolean | undefined;
}

export interface JsonApiObject {
  type: string;
  id: string;
  attributes: Record<string, any>;
  relationships?: Record<string, any>;
}

export interface JsonApiListResponse {
  data: JsonApiObject[];
  links?: {
    current_page: number;
    from: number;
    last_page: number;
    per_page: number;
    to: number;
    total: number;
  };
  included?: JsonApiObject[];
}

export interface JsonApiSingleResponse {
  data: JsonApiObject;
  included?: JsonApiObject[];
}

export interface ActionResponse {
  ResponseType: string;
  Attributes: Record<string, any>;
}

export interface AggregateParams {
  column: string[];
  group?: string[];
  filter?: string[];
  having?: string[];
  sort?: string;
  limit?: number;
  join?: string;
}

export interface DaptinHostConfig {
  name: string;
  endpoint: string;
  token?: string;
}

export interface DaptinCliConfig {
  current_context?: string;
  hosts: DaptinHostConfig[];
}
