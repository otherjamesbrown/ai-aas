const instances: any[] = [];

class MockPool {
  public config: any;
  public query: (sql: string) => Promise<string>;
  public end: () => Promise<void>;

  constructor(config: any) {
    this.config = config;
    this.query = async (sql: string) => sql;
    this.end = async () => {};
    instances.push(this);
  }
}

const Pool = MockPool;

export { MockPool, Pool, instances as mockPoolInstances };

export default {
  Pool,
};

