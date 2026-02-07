import axios, { AxiosInstance, AxiosError } from 'axios';
import {
  DaptinQueryParameters,
  JsonApiListResponse,
  JsonApiSingleResponse,
  ActionResponse,
  AggregateParams,
} from './types';

export class DaptinClient {
  private endpoint: string;
  private token: string | null;
  private http: AxiosInstance;
  private debug: boolean;

  constructor(endpoint: string, token?: string, debug: boolean = false) {
    this.endpoint = endpoint.replace(/\/+$/, '');
    this.token = token || null;
    this.debug = debug;

    this.http = axios.create({
      baseURL: this.endpoint,
      headers: {
        'Content-Type': 'application/vnd.api+json',
      },
      timeout: 30000,
    });

    this.http.interceptors.request.use((config) => {
      if (this.token) {
        config.headers['Authorization'] = `Bearer ${this.token}`;
      }
      return config;
    });

    if (debug) {
      this.http.interceptors.request.use((config) => {
        console.error(`[DEBUG] ${config.method?.toUpperCase()} ${config.baseURL}${config.url}`);
        if (config.params) console.error(`[DEBUG] Params:`, config.params);
        return config;
      });
      this.http.interceptors.response.use(
        (response) => {
          console.error(`[DEBUG] Response ${response.status}`);
          return response;
        },
        (error: AxiosError) => {
          console.error(`[DEBUG] Error ${error.response?.status || error.code}`);
          return Promise.reject(error);
        }
      );
    }
  }

  setToken(token: string): void {
    this.token = token;
  }

  getToken(): string | null {
    return this.token;
  }

  getEndpoint(): string {
    return this.endpoint;
  }

  // --- CRUD Operations ---

  async findAll(entity: string, params: DaptinQueryParameters = {}): Promise<JsonApiListResponse> {
    const response = await this.http.get(`/api/${entity}`, { params });
    return response.data;
  }

  async findOne(entity: string, referenceId: string, params: DaptinQueryParameters = {}): Promise<JsonApiSingleResponse> {
    const response = await this.http.get(`/api/${entity}/${referenceId}`, { params });
    return response.data;
  }

  async create(entity: string, attributes: Record<string, any>): Promise<JsonApiSingleResponse> {
    const response = await this.http.post(`/api/${entity}`, {
      data: {
        type: entity,
        attributes,
      },
    });
    return response.data;
  }

  async update(
    entity: string,
    referenceId: string,
    attributes: Record<string, any>,
    relationships?: Record<string, any>
  ): Promise<JsonApiSingleResponse> {
    const data: any = {
      type: entity,
      id: referenceId,
      attributes,
    };
    if (relationships) {
      data.relationships = relationships;
    }
    const response = await this.http.patch(`/api/${entity}/${referenceId}`, { data });
    return response.data;
  }

  async deleteOne(entity: string, referenceId: string): Promise<void> {
    await this.http.delete(`/api/${entity}/${referenceId}`);
  }

  // --- Actions ---

  async executeAction(
    entity: string,
    actionName: string,
    data: Record<string, any> = {}
  ): Promise<ActionResponse[]> {
    const response = await this.http.post(`/action/${entity}/${actionName}`, {
      attributes: data,
    });
    return response.data;
  }

  async executeActionOnInstance(
    entity: string,
    referenceId: string,
    actionName: string,
    data: Record<string, any> = {}
  ): Promise<ActionResponse[]> {
    const response = await this.http.post(`/action/${entity}/${referenceId}/${actionName}`, {
      attributes: data,
    });
    return response.data;
  }

  // --- Aggregation ---

  async aggregate(entity: string, params: AggregateParams): Promise<any> {
    const response = await this.http.post(`/aggregate/${entity}`, params);
    return response.data;
  }

  // --- Relations ---

  async getRelation(
    entity: string,
    referenceId: string,
    relation: string,
    params: DaptinQueryParameters = {}
  ): Promise<JsonApiListResponse> {
    const response = await this.http.get(`/api/${entity}/${referenceId}/${relation}`, { params });
    return response.data;
  }

  // --- Meta / Discovery ---

  async getWorlds(params: DaptinQueryParameters = {}): Promise<JsonApiListResponse> {
    return this.findAll('world', { 'page[size]': 500, ...params });
  }

  async getActions(params: DaptinQueryParameters = {}): Promise<JsonApiListResponse> {
    return this.findAll('action', { 'page[size]': 500, ...params });
  }

  async getMeta(query?: string): Promise<any> {
    const url = query ? `/meta?query=${encodeURIComponent(query)}` : '/meta';
    const response = await this.http.get(url);
    return response.data;
  }

  async getHealth(): Promise<any> {
    const response = await this.http.get('/health');
    return response.data;
  }
}

export { DaptinClient as default };
