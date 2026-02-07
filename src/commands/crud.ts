import { Command } from 'commander';
import chalk from 'chalk';
import { getContext, handleError, extractAttributes, filterColumns } from '../context';
import { DaptinQueryParameters } from '../client/types';

export function registerCrudCommands(program: Command): void {
  // --- LIST ---
  program
    .command('list <table>')
    .description('List records from a table')
    .option('--page-size <n>', 'Records per page', '10')
    .option('--page <n>', 'Page number (1-indexed)', '1')
    .option('--filter <json>', 'Filter conditions (JSON array)')
    .option('--sort <fields>', 'Sort fields (prefix with - for desc)')
    .option('--columns <cols>', 'Comma-separated columns to display')
    .option('--include <rels>', 'Include relationships')
    .option('--fields <fields>', 'Fields to request from server')
    .action(async (table: string, options: any, cmd: Command) => {
      try {
        const { client, renderer } = getContext(cmd);
        const params: DaptinQueryParameters = {
          'page[size]': parseInt(options.pageSize, 10),
          'page[number]': parseInt(options.page, 10),
        };

        if (options.filter) params['query'] = options.filter;
        if (options.sort) params['sort'] = options.sort;
        if (options.include) params['included_relations'] = options.include;
        if (options.fields) params['fields'] = options.fields;

        const response = await client.findAll(table, params);
        let data = extractAttributes(response.data);

        if (data.length === 0) {
          console.log(chalk.yellow('No records found.'));
          return;
        }

        const columns = options.columns
          ? options.columns.split(',').map((c: string) => c.trim())
          : undefined;

        if (columns) {
          data = filterColumns(data, columns);
        }

        renderer.renderArray(data, columns);

        // Show pagination info
        if (response.links) {
          const links = response.links as any;
          const total = links.total || data.length;
          const currentPage = links.current_page || parseInt(options.page, 10);
          const lastPage = links.last_page || 1;
          console.log(
            chalk.gray(`\nPage ${currentPage}/${lastPage} (${total} total records)`)
          );
        }
      } catch (error) {
        handleError(error);
      }
    });

  // --- GET ---
  program
    .command('get <table> <referenceId>')
    .description('Get a single record by reference_id')
    .option('--columns <cols>', 'Comma-separated columns to display')
    .option('--include <rels>', 'Include relationships')
    .action(async (table: string, referenceId: string, options: any, cmd: Command) => {
      try {
        const { client, renderer } = getContext(cmd);
        const params: DaptinQueryParameters = {};

        if (options.include) params['included_relations'] = options.include;

        const response = await client.findOne(table, referenceId, params);

        if (!response.data) {
          console.log(chalk.yellow('Record not found.'));
          return;
        }

        const attrs = response.data.attributes || {};
        const columns = options.columns
          ? options.columns.split(',').map((c: string) => c.trim())
          : undefined;

        renderer.renderObject(attrs, columns);
      } catch (error) {
        handleError(error);
      }
    });

  // --- CREATE ---
  program
    .command('create <table>')
    .description('Create a new record')
    .requiredOption('--data <json>', 'JSON object with record attributes')
    .action(async (table: string, options: any, cmd: Command) => {
      try {
        const { client, renderer } = getContext(cmd);
        let attributes: Record<string, any>;

        try {
          attributes = JSON.parse(options.data);
        } catch {
          console.error(chalk.red('Invalid JSON in --data flag.'));
          process.exit(1);
        }

        const response = await client.create(table, attributes);
        const attrs = response.data?.attributes || {};

        console.log(chalk.green('Record created successfully.'));
        renderer.renderObject(attrs);
      } catch (error) {
        handleError(error);
      }
    });

  // --- UPDATE ---
  program
    .command('update <table> <referenceId>')
    .description('Update an existing record')
    .requiredOption('--data <json>', 'JSON object with attributes to update')
    .action(async (table: string, referenceId: string, options: any, cmd: Command) => {
      try {
        const { client, renderer } = getContext(cmd);
        let attributes: Record<string, any>;

        try {
          attributes = JSON.parse(options.data);
        } catch {
          console.error(chalk.red('Invalid JSON in --data flag.'));
          process.exit(1);
        }

        const response = await client.update(table, referenceId, attributes);
        const attrs = response.data?.attributes || {};

        console.log(chalk.green('Record updated successfully.'));
        renderer.renderObject(attrs);
      } catch (error) {
        handleError(error);
      }
    });

  // --- DELETE ---
  program
    .command('delete <table> <referenceId>')
    .description('Delete a record by reference_id')
    .action(async (table: string, referenceId: string, _options: any, cmd: Command) => {
      try {
        const { client } = getContext(cmd);
        await client.deleteOne(table, referenceId);
        console.log(chalk.green(`Record ${referenceId} deleted from "${table}".`));
      } catch (error) {
        handleError(error);
      }
    });

  // --- RELATION ---
  program
    .command('relation <table> <referenceId> <relation>')
    .description('Query related records through a relationship')
    .option('--page-size <n>', 'Records per page', '10')
    .option('--page <n>', 'Page number', '1')
    .option('--columns <cols>', 'Comma-separated columns to display')
    .action(
      async (
        table: string,
        referenceId: string,
        relation: string,
        options: any,
        cmd: Command
      ) => {
        try {
          const { client, renderer } = getContext(cmd);
          const params: DaptinQueryParameters = {
            'page[size]': parseInt(options.pageSize, 10),
            'page[number]': parseInt(options.page, 10),
          };

          const response = await client.getRelation(table, referenceId, relation, params);

          if (Array.isArray(response.data)) {
            const data = extractAttributes(response.data);
            if (data.length === 0) {
              console.log(chalk.yellow('No related records found.'));
              return;
            }
            const columns = options.columns
              ? options.columns.split(',').map((c: string) => c.trim())
              : undefined;
            renderer.renderArray(columns ? filterColumns(data, columns) : data, columns);
          } else if (response.data) {
            const attrs = (response.data as any).attributes || {};
            renderer.renderObject(attrs);
          } else {
            console.log(chalk.yellow('No related records found.'));
          }
        } catch (error) {
          handleError(error);
        }
      }
    );
}
