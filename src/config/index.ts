import * as fs from 'fs';
import * as path from 'path';
import * as os from 'os';
import * as yaml from 'js-yaml';
import { DaptinCliConfig, DaptinHostConfig } from '../client/types';

export class ConfigManager {
  private configPath: string;
  private config: DaptinCliConfig;

  constructor(configPath?: string) {
    this.configPath = configPath || ConfigManager.getDefaultConfigPath();
    this.config = this.load();
  }

  static getDefaultConfigPath(): string {
    const envPath = process.env.DAPTIN_CLI_CONFIG;
    if (envPath) return envPath;

    const daptinDir = path.join(os.homedir(), '.daptin');
    if (!fs.existsSync(daptinDir)) {
      fs.mkdirSync(daptinDir, { recursive: true });
    }
    return path.join(daptinDir, 'config.yaml');
  }

  private load(): DaptinCliConfig {
    try {
      if (fs.existsSync(this.configPath)) {
        const content = fs.readFileSync(this.configPath, 'utf-8');
        const parsed = yaml.load(content) as any;
        if (!parsed) return { hosts: [] };

        // Support both old Go config format (PascalCase) and new format (snake_case)
        return {
          current_context: parsed.current_context || parsed.CurrentContext || undefined,
          hosts: (parsed.hosts || parsed.Hosts || []).map((h: any) => ({
            name: h.name || h.Name,
            endpoint: h.endpoint || h.Endpoint,
            token: h.token || h.Token || undefined,
          })),
        };
      }
    } catch {
      // Config file doesn't exist or is invalid, start fresh
    }
    return { hosts: [] };
  }

  save(): void {
    const content = yaml.dump(this.config, { lineWidth: -1 });
    const dir = path.dirname(this.configPath);
    if (!fs.existsSync(dir)) {
      fs.mkdirSync(dir, { recursive: true });
    }
    fs.writeFileSync(this.configPath, content, 'utf-8');
  }

  getConfig(): DaptinCliConfig {
    return this.config;
  }

  getCurrentContext(): DaptinHostConfig | undefined {
    if (!this.config.current_context) return undefined;
    return this.config.hosts.find((h) => h.name === this.config.current_context);
  }

  addHost(host: DaptinHostConfig): void {
    const existingIdx = this.config.hosts.findIndex((h) => h.name === host.name);
    if (existingIdx >= 0) {
      this.config.hosts[existingIdx] = host;
    } else {
      this.config.hosts.push(host);
    }
    if (!this.config.current_context) {
      this.config.current_context = host.name;
    }
    this.save();
  }

  removeHost(name: string): boolean {
    const index = this.config.hosts.findIndex((h) => h.name === name);
    if (index < 0) return false;
    this.config.hosts.splice(index, 1);
    if (this.config.current_context === name) {
      this.config.current_context =
        this.config.hosts.length > 0 ? this.config.hosts[0].name : undefined;
    }
    this.save();
    return true;
  }

  setCurrentContext(name: string): boolean {
    const host = this.config.hosts.find((h) => h.name === name);
    if (!host) return false;
    this.config.current_context = name;
    this.save();
    return true;
  }

  updateHostToken(nameOrEndpoint: string, token: string): void {
    const host = this.config.hosts.find(
      (h) => h.name === nameOrEndpoint || h.endpoint === nameOrEndpoint
    );
    if (host) {
      host.token = token;
      this.save();
    }
  }

  getConfigPath(): string {
    return this.configPath;
  }
}
