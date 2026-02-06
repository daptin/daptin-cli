import { Command } from 'commander';
import chalk from 'chalk';
import { getContext, handleError, extractAttributes, filterColumns } from '../context';

export function registerDescribeCommands(program: Command): void {
  program
    .command('tables')
    .description('List all tables on the server')
    .option('--columns <cols>', 'Comma-separated columns to display')
    .action(async (options: any, cmd: Command) => {
      try {
        const { client, renderer } = getContext(cmd);
        const response = await client.getWorlds();
        let data = extractAttributes(response.data);

        const defaultColumns = ['table_name', 'is_top_level', 'is_hidden'];
        const columns = options.columns
          ? options.columns.split(',').map((c: string) => c.trim())
          : defaultColumns;

        data = filterColumns(data, columns);

        // Sort by table_name for readability
        data.sort((a, b) => (a.table_name || '').localeCompare(b.table_name || ''));

        renderer.renderArray(data, columns);
        console.log(chalk.gray(`\n${data.length} tables found.`));
      } catch (error) {
        handleError(error);
      }
    });

  program
    .command('describe <table>')
    .description('Show table schema (columns and actions)')
    .option('--columns <cols>', 'Comma-separated column fields to display')
    .action(async (tableName: string, options: any, cmd: Command) => {
      try {
        const { client, renderer } = getContext(cmd);

        // Fetch all worlds to find the target table
        const worldsResponse = await client.getWorlds();
        const worlds = extractAttributes(worldsResponse.data);
        const world = worlds.find((w) => w.table_name === tableName);

        if (!world) {
          console.error(chalk.red(`Table "${tableName}" not found.`));
          const available = worlds
            .map((w) => w.table_name)
            .filter(Boolean)
            .sort();
          if (available.length > 0) {
            console.error(chalk.gray('Available tables:'), available.join(', '));
          }
          process.exit(1);
        }

        // Parse the world schema
        let schema: any;
        try {
          schema = JSON.parse(world.world_schema_json);
        } catch {
          console.error(chalk.red('Could not parse table schema.'));
          process.exit(1);
        }

        // Display columns
        console.log(chalk.cyan.bold(`\nTable: ${tableName}`));
        console.log(chalk.cyan('Columns:'));

        const columnData = (schema.Columns || []).map((col: any) => ({
          ColumnName: col.ColumnName || col.Name,
          ColumnType: col.ColumnType,
          DataType: col.DataType || '',
          IsNullable: col.IsNullable ? 'yes' : 'no',
          IsUnique: col.IsUnique ? 'yes' : 'no',
          IsIndexed: col.IsIndexed ? 'yes' : 'no',
          IsForeignKey: col.IsForeignKey ? 'yes' : 'no',
        }));

        const colDisplayCols = options.columns
          ? options.columns.split(',').map((c: string) => c.trim())
          : ['ColumnName', 'ColumnType', 'IsNullable'];

        renderer.renderArray(filterColumns(columnData, colDisplayCols), colDisplayCols);

        // Display relations if any
        if (schema.Relations && schema.Relations.length > 0) {
          console.log(chalk.cyan(`\nRelations: ${schema.Relations.length}`));
          const relData = schema.Relations.map((rel: any) => ({
            Subject: rel.Subject || rel.SubjectName,
            Relation: rel.Relation,
            Object: rel.Object || rel.ObjectName,
          }));
          renderer.renderArray(relData);
        }

        // Display associated actions
        const actionsResponse = await client.getActions();
        const allActions = extractAttributes(actionsResponse.data);
        const worldActions = allActions.filter(
          (a) => a.world_id === world.reference_id
        );

        if (worldActions.length > 0) {
          console.log(chalk.cyan(`\nActions: ${worldActions.length}`));
          const actionData = filterColumns(worldActions, [
            'action_name',
            'label',
            'reference_id',
          ]);
          renderer.renderArray(actionData, ['action_name', 'label', 'reference_id']);
        } else {
          console.log(chalk.gray('\nNo actions associated with this table.'));
        }

        // Display state machines if enabled
        if (schema.StateMachines && schema.StateMachines.length > 0) {
          console.log(chalk.cyan(`\nState Machines: ${schema.StateMachines.length}`));
          const smData = schema.StateMachines.map((sm: any) => ({
            Name: sm.Name,
            Label: sm.Label || '',
            InitialState: sm.InitialState,
            Events: (sm.Events || []).length,
          }));
          renderer.renderArray(smData);
        }
      } catch (error) {
        handleError(error);
      }
    });
}
