// Global test setup for Vitest
// Per TEST_PRACTICES.md: localStorage isolation (3.8)
beforeEach(() => {
  localStorage.clear()
})
