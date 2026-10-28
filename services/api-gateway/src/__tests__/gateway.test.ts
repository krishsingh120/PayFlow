import app from '../index';

describe('API Gateway Smoke & Configuration', () => {
  it('should export the express app instance', () => {
    expect(app).toBeDefined();
    expect(typeof app.use).toBe('function');
    expect(typeof app.listen).toBe('function');
  });

  it('should have standard middleware registered', () => {
    // Verify Express router stack includes handlers
    expect((app as any)._router).toBeDefined();
    expect((app as any)._router.stack.length).toBeGreaterThan(0);
  });
});
