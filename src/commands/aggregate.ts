import { Command } from 'commander';
import chalk from 'chalk';
import { getContext, handleError } from '../context';
import { AggregateParams } from '../client/types';

export function registerAggregateCommands(program: Command): void {
  program
    .command('aggregate <table>')
    .description('Run an aggregation query on a table')
    .requiredOption('--columns <cols>', 'Aggregation columns (comma-separated, e.g. "count,sum(price),avg(price)")')
    .option('--group <cols>', 'Group by columns (comma-separated)')
    .option('--filter <conditions>', 'Filter conditions (comma-separated, e.g. "gt(price,100)")')
    .option('--having <conditions>', 'Having conditions (comma-separated)')
    .option('--sort <fields>', 'Sort results')
    .option('--limit <n>', 'Limit number of results')
    .option('--join <spec>', 'Join specification (e.g. "table@condition")')
    .action(async (table: string, options: any, cmd: Command) => {
      try {
        const { client, renderer } = getContext(cmd);

        const params: AggregateParams = {
          column: options.columns.split(',').map((c: string) => c.trim()),
        };

        if (options.group) {
          params.group = options.group.split(',').map((c: string) => c.trim());
        }
        if (options.filter) {
          params.filter = options.filter.split(',').map((c: string) => c.trim());
        }
        if (options.having) {
          params.having = options.having.split(',').map((c: string) => c.trim());
        }
        if (options.sort) {
          params.sort = options.sort;
        }
        if (options.limit) {
          params.limit = parseInt(options.limit, 10);
        }
        if (options.join) {
          params.join = options.join;
        }

        const result = await client.aggregate(table, params);

        if (Array.isArray(result)) {
          if (result.length === 0) {
            console.log(chalk.yellow('No aggregation results.'));
            return;
          }
          renderer.renderArray(result);
        } else if (typeof result === 'object' && result !== null) {
          // Single result object
          if (result.data && Array.isArray(result.data)) {
            renderer.renderArray(result.data);
          } else {
            renderer.renderObject(result);
          }
        } else {
          console.log(result);
        }
      } catch (error) {
        handleError(error);
      }
    });
}
