import { Command } from 'commander';
import chalk from 'chalk';
import inquirer from 'inquirer';
import { getContext, handleError } from '../context';
import { ActionResponse } from '../client/types';
import { ConfigManager } from '../config';

/**
 * Process action responses from signin/signup.
 * Handles client.store.set (saves token), client.notify, etc.
 */
function handleAuthResponse(
  responses: ActionResponse[],
  configManager: ConfigManager,
  contextName?: string
): void {
  for (const response of responses) {
    switch (response.ResponseType) {
      case 'client.store.set':
        if (response.Attributes.key === 'token') {
          const token = response.Attributes.value;
          if (contextName) {
            configManager.updateHostToken(contextName, token);
          }
          console.log(chalk.green('Authentication successful. Token saved.'));
        }
        break;
      case 'client.notify':
        const msgType = response.Attributes.type || 'info';
        const msg = response.Attributes.message || '';
        if (msgType === 'error') {
          console.error(chalk.red('Server:'), msg);
        } else {
          console.log(chalk.blue('Server:'), msg);
        }
        break;
      case 'client.redirect':
        console.log(chalk.gray(`Redirect: ${response.Attributes.location}`));
        break;
      default:
        break;
    }
  }
}

export function registerAuthCommands(program: Command): void {
  program
    .command('signup <email>')
    .description('Create a new account on the Daptin server')
    .action(async (email: string, _options: any, cmd: Command) => {
      try {
        const { client, configManager } = getContext(cmd);
        const answers = await inquirer.prompt([
          {
            type: 'password',
            name: 'password',
            message: 'Password:',
            mask: '*',
          },
          {
            type: 'password',
            name: 'passwordConfirm',
            message: 'Confirm password:',
            mask: '*',
          },
        ]);

        if (answers.password !== answers.passwordConfirm) {
          console.error(chalk.red('Passwords do not match.'));
          process.exit(1);
        }

        const responses = await client.executeAction('user_account', 'signup', {
          email,
          password: answers.password,
          passwordConfirm: answers.passwordConfirm,
        });

        const currentCtx = configManager.getCurrentContext();
        handleAuthResponse(responses, configManager, currentCtx?.name || currentCtx?.endpoint);
      } catch (error) {
        handleError(error);
      }
    });

  program
    .command('signin <email>')
    .description('Sign in to the Daptin server')
    .action(async (email: string, _options: any, cmd: Command) => {
      try {
        const { client, configManager } = getContext(cmd);
        const answers = await inquirer.prompt([
          {
            type: 'password',
            name: 'password',
            message: 'Password:',
            mask: '*',
          },
        ]);

        const responses = await client.executeAction('user_account', 'signin', {
          email,
          password: answers.password,
        });

        const currentCtx = configManager.getCurrentContext();
        handleAuthResponse(responses, configManager, currentCtx?.name || currentCtx?.endpoint);
      } catch (error) {
        handleError(error);
      }
    });

  program
    .command('signin-2fa <email>')
    .description('Sign in with two-factor authentication')
    .action(async (email: string, _options: any, cmd: Command) => {
      try {
        const { client, configManager } = getContext(cmd);
        const answers = await inquirer.prompt([
          {
            type: 'password',
            name: 'password',
            message: 'Password:',
            mask: '*',
          },
          {
            type: 'input',
            name: 'otp',
            message: 'OTP code:',
          },
        ]);

        const responses = await client.executeAction('user_account', 'signin_with_2fa', {
          email,
          password: answers.password,
          otp: answers.otp,
        });

        const currentCtx = configManager.getCurrentContext();
        handleAuthResponse(responses, configManager, currentCtx?.name || currentCtx?.endpoint);
      } catch (error) {
        handleError(error);
      }
    });

  program
    .command('whoami')
    .description('Show current authentication status')
    .action(async (_options: any, cmd: Command) => {
      try {
        const { client, configManager } = getContext(cmd);
        const currentCtx = configManager.getCurrentContext();

        console.log(chalk.cyan('Server:'), client.getEndpoint());

        const token = client.getToken();
        if (!token) {
          console.log(chalk.yellow('Not authenticated.'));
          return;
        }

        // Decode JWT payload (base64 middle segment)
        try {
          const parts = token.split('.');
          if (parts.length === 3) {
            const payload = JSON.parse(Buffer.from(parts[1], 'base64').toString('utf-8'));
            console.log(chalk.cyan('Email:'), payload.email || 'N/A');
            console.log(chalk.cyan('User ID:'), payload.sub || payload.id || 'N/A');
            if (payload.exp) {
              const expDate = new Date(payload.exp * 1000);
              const isExpired = expDate < new Date();
              console.log(
                chalk.cyan('Expires:'),
                expDate.toISOString(),
                isExpired ? chalk.red('(expired)') : chalk.green('(valid)')
              );
            }
          }
        } catch {
          console.log(chalk.yellow('Token present but could not decode claims.'));
        }

        if (currentCtx) {
          console.log(chalk.cyan('Context:'), currentCtx.name);
        }
      } catch (error) {
        handleError(error);
      }
    });
}
