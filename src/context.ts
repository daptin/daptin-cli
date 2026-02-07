import { Command } from 'commander';
import chalk from 'chalk';
import { DaptinClient } from './client';
import { ConfigManager } from './config';
import { createRenderer, Renderer } from './output';

export interface CliContext {
  client: DaptinClient;
  configManager: ConfigManager;
  renderer: Renderer;
}

/**
 * Build a CLI context from a command's options.
 * Creates the DaptinClient, ConfigManager, and Renderer based on CLI flags and config file.
 */
export function getContext(cmd: Command): CliContext {
  const opts = cmd.optsWithGlobals();
  const configManager = new ConfigManager(opts.config);
  const renderer = createRenderer(opts.output || 'table');

  const context = configManager.getCurrentContext();
  const endpoint = opts.endpoint || context?.endpoint;
  const token = opts.token || context?.token;

  if (!endpoint) {
    console.error(chalk.red('Error: No endpoint configured.'));
    console.error('Use "daptin-cli config add <name> <endpoint>" to add a server,');
    console.error('or pass --endpoint <url> on the command line.');
    process.exit(1);
  }

  const client = new DaptinClient(endpoint, token, opts.debug);
  return { client, configManager, renderer };
}

/**
 * Standard error handler for CLI commands.
 * Formats axios errors and other exceptions into user-friendly messages.
 */
export function handleError(error: any): void {
  if (error.response) {
    const status = error.response.status;
    const data = error.response.data;
    const message =
      typeof data === 'string'
        ? data
        : data?.message || data?.errors?.[0]?.title || JSON.stringify(data);
    console.error(chalk.red(`Error ${status}:`), message);
  } else if (error.code === 'ECONNREFUSED') {
    console.error(
      chalk.red('Connection refused.'),
      'Is the Daptin server running at the configured endpoint?'
    );
  } else if (error.code === 'ENOTFOUND') {
    console.error(chalk.red('Host not found.'), 'Check the endpoint URL.');
  } else if (error.code === 'ETIMEDOUT') {
    console.error(chalk.red('Connection timed out.'));
  } else if (error.message) {
    console.error(chalk.red('Error:'), error.message);
  } else {
    console.error(chalk.red('Unknown error occurred.'));
  }
  process.exit(1);
}

/**
 * Extract attributes from JSON:API response objects.
 */
export function extractAttributes(
  objects: Array<{ attributes?: Record<string, any> }>
): Record<string, any>[] {
  return objects.map((obj) => obj.attributes || {});
}

/**
 * Filter columns from a data array.
 */
export function filterColumns(
  data: Record<string, any>[],
  columns: string[]
): Record<string, any>[] {
  if (!columns || columns.length === 0) return data;
  return data.map((row) => {
    const filtered: Record<string, any> = {};
    for (const col of columns) {
      if (col in row) filtered[col] = row[col];
    }
    return filtered;
  });
}
