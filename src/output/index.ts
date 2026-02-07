import Table from 'cli-table3';
import chalk from 'chalk';

export type OutputFormat = 'table' | 'json';

export interface Renderer {
  renderArray(data: Record<string, any>[], columns?: string[]): void;
  renderObject(data: Record<string, any>, columns?: string[]): void;
}

export class TableRenderer implements Renderer {
  renderArray(data: Record<string, any>[], columns?: string[]): void {
    if (!data || data.length === 0) {
      console.log(chalk.yellow('No data found.'));
      return;
    }

    const headers = columns && columns.length > 0 ? columns : Object.keys(data[0]);

    const table = new Table({
      head: headers.map((h) => chalk.cyan(h)),
      style: { head: [], border: [] },
      wordWrap: true,
    });

    for (const row of data) {
      table.push(
        headers.map((h) => {
          const val = row[h];
          if (val === null || val === undefined) return '';
          const str = typeof val === 'object' ? JSON.stringify(val) : String(val);
          return str.length > 60 ? str.substring(0, 57) + '...' : str;
        })
      );
    }

    console.log(table.toString());
  }

  renderObject(data: Record<string, any>, columns?: string[]): void {
    if (!data || Object.keys(data).length === 0) {
      console.log(chalk.yellow('No data found.'));
      return;
    }

    const keys = columns && columns.length > 0 ? columns : Object.keys(data);

    const table = new Table({
      style: { head: [], border: [] },
    });

    for (const key of keys) {
      if (!(key in data)) continue;
      const val = data[key];
      let str: string;
      if (val === null || val === undefined) {
        str = '';
      } else if (typeof val === 'object') {
        str = JSON.stringify(val, null, 2);
      } else {
        str = String(val);
      }
      if (str.length > 100) str = str.substring(0, 97) + '...';
      table.push({ [chalk.cyan(key)]: str });
    }

    console.log(table.toString());
  }
}

export class JsonRenderer implements Renderer {
  renderArray(data: Record<string, any>[], columns?: string[]): void {
    if (columns && columns.length > 0) {
      data = data.map((row) => {
        const filtered: Record<string, any> = {};
        for (const col of columns) {
          if (col in row) filtered[col] = row[col];
        }
        return filtered;
      });
    }
    console.log(JSON.stringify(data, null, 2));
  }

  renderObject(data: Record<string, any>, columns?: string[]): void {
    if (columns && columns.length > 0) {
      const filtered: Record<string, any> = {};
      for (const col of columns) {
        if (col in data) filtered[col] = data[col];
      }
      data = filtered;
    }
    console.log(JSON.stringify(data, null, 2));
  }
}

export function createRenderer(format: OutputFormat): Renderer {
  switch (format) {
    case 'json':
      return new JsonRenderer();
    case 'table':
    default:
      return new TableRenderer();
  }
}
