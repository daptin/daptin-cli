import { Command } from 'commander';
import chalk from 'chalk';
import { ConfigManager } from '../config';
import { createRenderer } from '../output';

export function registerConfigCommands(program: Command): void {
  const config = program
    .command('config')
    .description('Manage server configurations');

  config
    .command('add <name> <endpoint>')
    .description('Add a new server configuration')
    .action((name: string, endpoint: string, _options: any, cmd: Command) => {
      const opts = cmd.optsWithGlobals();
      const configManager = new ConfigManager(opts.config);
      configManager.addHost({ name, endpoint: endpoint.replace(/\/+$/, '') });
      console.log(chalk.green(`Added server "${name}" (${endpoint})`));
      if (configManager.getConfig().current_context === name) {
        console.log(chalk.gray(`Set as active context.`));
      }
    });

  config
    .command('use <name>')
    .description('Switch the active server context')
    .action((name: string, _options: any, cmd: Command) => {
      const opts = cmd.optsWithGlobals();
      const configManager = new ConfigManager(opts.config);
      if (configManager.setCurrentContext(name)) {
        console.log(chalk.green(`Switched to context "${name}".`));
      } else {
        console.error(chalk.red(`Context "${name}" not found.`));
        const hosts = configManager.getConfig().hosts;
        if (hosts.length > 0) {
          console.error('Available contexts:', hosts.map((h) => h.name).join(', '));
        }
        process.exit(1);
      }
    });

  config
    .command('list')
    .description('List all server configurations')
    .action((_options: any, cmd: Command) => {
      const opts = cmd.optsWithGlobals();
      const configManager = new ConfigManager(opts.config);
      const cfg = configManager.getConfig();
      const renderer = createRenderer(opts.output || 'table');

      if (cfg.hosts.length === 0) {
        console.log(chalk.yellow('No servers configured.'));
        console.log('Use "daptin-cli config add <name> <endpoint>" to add one.');
        return;
      }

      const data = cfg.hosts.map((h) => ({
        name: h.name,
        endpoint: h.endpoint,
        active: h.name === cfg.current_context ? '*' : '',
        authenticated: h.token ? 'yes' : 'no',
      }));

      renderer.renderArray(data);
    });

  config
    .command('remove <name>')
    .description('Remove a server configuration')
    .action((name: string, _options: any, cmd: Command) => {
      const opts = cmd.optsWithGlobals();
      const configManager = new ConfigManager(opts.config);
      if (configManager.removeHost(name)) {
        console.log(chalk.green(`Removed context "${name}".`));
      } else {
        console.error(chalk.red(`Context "${name}" not found.`));
        process.exit(1);
      }
    });

  config
    .command('show')
    .description('Show the current active configuration')
    .action((_options: any, cmd: Command) => {
      const opts = cmd.optsWithGlobals();
      const configManager = new ConfigManager(opts.config);
      const current = configManager.getCurrentContext();

      if (!current) {
        console.log(chalk.yellow('No active context set.'));
        console.log('Use "daptin-cli config add <name> <endpoint>" to add a server.');
        return;
      }

      console.log(chalk.cyan('Active context:'));
      console.log(`  Name:     ${current.name}`);
      console.log(`  Endpoint: ${current.endpoint}`);
      console.log(`  Auth:     ${current.token ? 'authenticated' : 'not authenticated'}`);
      console.log(`  Config:   ${configManager.getConfigPath()}`);
    });
}
