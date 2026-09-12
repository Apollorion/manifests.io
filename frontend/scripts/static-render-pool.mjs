import { Worker, isMainThread, parentPort, workerData } from 'node:worker_threads';
import { renderStaticPage } from './static-render.mjs';

export class StaticRenderPool {
  constructor({ template, rendererURL, size }) {
    if (!Number.isInteger(size) || size < 1 || size > 4) throw new Error('Static render pool size must be between one and four');
    this.workers = [];
    this.queue = [];
    this.failure = undefined;
    this.closed = false;
    for (let index = 0; index < size; index++) {
      const worker = new Worker(new URL(import.meta.url), { workerData: { template, rendererURL } });
      const slot = { worker, job: undefined };
      worker.on('message', message => {
        const job = slot.job;
        slot.job = undefined;
        if (!job) return;
        if (message.error) job.reject(new Error(message.error));
        else job.resolve(message);
        this.dispatch();
      });
      worker.on('error', error => this.fail(error));
      worker.on('exit', () => {
        if (!this.closed) this.fail(new Error('Static render worker exited before completion'));
      });
      this.workers.push(slot);
    }
  }

  fail(error) {
    if (this.failure) return;
    this.failure = error;
    for (const slot of this.workers) {
      slot.job?.reject(error);
      slot.job = undefined;
    }
    for (const job of this.queue.splice(0)) job.reject(error);
  }

  dispatch() {
    if (this.failure || this.closed) return;
    for (const slot of this.workers) {
      if (!slot.job && this.queue.length) {
        slot.job = this.queue.shift();
        slot.worker.postMessage(slot.job.record);
      }
    }
  }

  render(record) {
    if (this.failure || this.closed) return Promise.reject(this.failure || new Error('Static render pool is closed'));
    return new Promise((resolve, reject) => {
      this.queue.push({ record, resolve, reject });
      this.dispatch();
    });
  }

  async close() {
    this.closed = true;
    this.fail(new Error('Static render pool closed'));
    await Promise.all(this.workers.map(slot => slot.worker.terminate()));
  }
}

if (!isMainThread) {
  const { render } = await import(workerData.rendererURL);
  parentPort.on('message', record => {
    try {
      parentPort.postMessage(renderStaticPage(workerData.template, record, render));
    } catch (error) {
      parentPort.postMessage({ error: error instanceof Error ? error.message : 'Static rendering failed' });
    }
  });
}
