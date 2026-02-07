import { Command } from 'commander';
import chalk from 'chalk';
import { getContext, handleError, extractAttributes, filterColumns } from '../context';
import { ActionResponse } from '../client/types';

/**
 * Process and display action responses from the server.
 */
function displayActionResponses(responses: ActionResponse[]): void {
  if (!Array.isArray(responses)) {
    console.log(chalk.yellow('No response from action.'));
    return;
  }

  for (const response of responses) {
    switch (response.ResponseType) {
      case 'client.notify': {
        const msgType = response.Attributes.type || 'info';
        const msg = response.Attributes.message || '';
        if (msgType === 'error') {
          console.error(chalk.red('Notification:'), msg);
        } else if (msgType === 'warning') {
          console.log(chalk.yellow('Notification:'), msg);
        } else {
          console.log(chalk.green('Notification:'), msg);
        }
        break;
      }
      case 'client.store.set':
        console.log(
          chalk.blue('Store:'),
          `${response.Attributes.key} = ${String(response.Attributes.value).substring(0, 50)}...`
        );
        break;
      case 'client.redirect':
        console.log(chalk.blue('Redirect:'), response.Attributes.location);
        break;
      case 'client.file.download':
        console.log(
          chalk.blue('File download:'),
          response.Attributes.name || 'unnamed',
          `(${response.Attributes.contentType || 'unknown type'})`
        );
        // In CLI context, write to stdout or file
        if (response.Attributes.content) {
          const content = Buffer.from(response.Attributes.content, 'base64');
          process.stdout.write(content);
        }
        break;
      default:
        console.log(chalk.gray(`Response [${response.ResponseType}]:`), response.Attributes);
        break;
    }
  }
}

export function registerActionCommands(program: Command): void {
  program
    .command('actions [table]')
    .description('List available actions, optionally filtered by table')
    .option('--columns <cols>', 'Comma-separated columns to display')
    .action(async (table: string | undefined, options: any, cmd: Command) => {
      try {
        const { client, renderer } = getContext(cmd);
        const actionsResponse = await client.getActions();
        let actions = extractAttributes(actionsResponse.data);

        if (table) {
          // Get the world reference_id for filtering
          const worldsResponse = await client.getWorlds();
          const worlds = extractAttributes(worldsResponse.data);
          const world = worlds.find((w) => w.table_name === table);

          if (!world) {
            console.error(chalk.red(`Table "${table}" not found.`));
            process.exit(1);
          }

          actions = actions.filter((a) => a.world_id === world.reference_id);
        }

        if (actions.length === 0) {
          console.log(chalk.yellow('No actions found.'));
          return;
        }

        const defaultCols = ['action_name', 'label', 'instance_optional', 'reference_id'];
        const columns = options.columns
          ? options.columns.split(',').map((c: string) => c.trim())
          : defaultCols;

        actions = filterColumns(actions, columns);
        actions.sort((a, b) =>
          (a.action_name || '').localeCompare(b.action_name || '')
        );
        renderer.renderArray(actions, columns);
        console.log(chalk.gray(`\n${actions.length} actions found.`));
      } catch (error) {
        handleError(error);
      }
    });

  program
    .command('action-describe <table> <actionName>')
    .description('Show the schema of an action (input/output fields)')
    .action(async (table: string, actionName: string, _options: any, cmd: Command) => {
      try {
        const { client, renderer } = getContext(cmd);

        // Find the world
        const worldsResponse = await client.getWorlds();
        const worlds = extractAttributes(worldsResponse.data);
        const world = worlds.find((w) => w.table_name === table);

        if (!world) {
          console.error(chalk.red(`Table "${table}" not found.`));
          process.exit(1);
        }

        // Find the action
        const actionsResponse = await client.getActions();
        const allActions = extractAttributes(actionsResponse.data);
        const action = allActions.find(
          (a) =>
            a.world_id === world.reference_id && a.action_name === actionName
        );

        if (!action) {
          console.error(chalk.red(`Action "${actionName}" not found on table "${table}".`));
          const tableActions = allActions
            .filter((a) => a.world_id === world.reference_id)
            .map((a) => a.action_name);
          if (tableActions.length > 0) {
            console.error(chalk.gray('Available actions:'), tableActions.join(', '));
          }
          process.exit(1);
        }

        // Parse action schema
        let schema: any;
        try {
          schema = JSON.parse(action.action_schema);
        } catch {
          console.error(chalk.red('Could not parse action schema.'));
          process.exit(1);
        }

        console.log(chalk.cyan.bold(`\nAction: ${actionName}`));
        console.log(chalk.cyan(`Table: ${table}`));
        console.log(chalk.cyan(`Label: ${action.label || ''}`));

        // Display input fields
        const inFields = schema.InFields || [];
        console.log(chalk.cyan(`\nInput Fields: ${inFields.length}`));
        if (inFields.length > 0) {
          const fieldData = inFields.map((f: any) => ({
            Name: f.ColumnName || f.Name,
            Type: f.ColumnType,
            Nullable: f.IsNullable ? 'yes' : 'no',
          }));
          renderer.renderArray(fieldData);
        }

        // Display output fields
        const outFields = schema.OutFields || [];
        console.log(chalk.cyan(`\nOutput Fields: ${outFields.length}`));
        if (outFields.length > 0) {
          const outData = outFields.map((f: any, i: number) => ({
            Index: i,
            Type: f.Type,
            Attributes: JSON.stringify(f.Attributes || {}),
          }));
          renderer.renderArray(outData);
        }
      } catch (error) {
        handleError(error);
      }
    });

  program
    .command('execute <table> <actionName>')
    .description('Execute an action on a table')
    .option('--data <json>', 'JSON object with action parameters', '{}')
    .option('--id <referenceId>', 'Record reference_id for instance-level actions')
    .action(async (table: string, actionName: string, options: any, cmd: Command) => {
      try {
        const { client } = getContext(cmd);

        let data: Record<string, any>;
        try {
          data = JSON.parse(options.data);
        } catch {
          console.error(chalk.red('Invalid JSON in --data flag.'));
          process.exit(1);
        }

        let responses: ActionResponse[];
        if (options.id) {
          responses = await client.executeActionOnInstance(
            table,
            options.id,
            actionName,
            data
          );
        } else {
          responses = await client.executeAction(table, actionName, data);
        }

        displayActionResponses(responses);
      } catch (error) {
        handleError(error);
      }
    });
}
