#!/usr/bin/env node

import { Command } from 'commander';
import { registerConfigCommands } from './commands/config';
import { registerAuthCommands } from './commands/auth';
import { registerDescribeCommands } from './commands/describe';
import { registerCrudCommands } from './commands/crud';
import { registerActionCommands } from './commands/action';
import { registerAggregateCommands } from './commands/aggregate';

const program = new Command();

program
  .name('daptin-cli')
  .description('Command-line interface for Daptin Backend-as-a-Service')
  .version('0.1.0')
  .option('-e, --endpoint <url>', 'Daptin server endpoint')
  .option('-c, --config <path>', 'Config file path')
  .option('-o, --output <format>', 'Output format (table, json)', 'table')
  .option('-t, --token <token>', 'Auth token')
  .option('--debug', 'Enable debug output', false);

// Register all command groups
registerConfigCommands(program);
registerAuthCommands(program);
registerDescribeCommands(program);
registerCrudCommands(program);
registerActionCommands(program);
registerAggregateCommands(program);

program.parse(process.argv);
