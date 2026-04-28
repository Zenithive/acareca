// Shared configuration for K6 tests

export const config = {
  // Base URLs for different environments
  environments: {
    local: 'http://localhost:8080',
    dev: 'https://acareca-bam8.onrender.com',
    staging: 'https://staging.example.com',
    production: 'https://api.example.com',
  },
  
  // Test users (create these in your database before running tests)
  testUsers: [
    { email: 'loadtest1@example.com', password: 'LoadTest@123' },
    { email: 'loadtest2@example.com', password: 'LoadTest@123' },
    { email: 'loadtest3@example.com', password: 'LoadTest@123' },
    { email: 'loadtest4@example.com', password: 'LoadTest@123' },
    { email: 'loadtest5@example.com', password: 'LoadTest@123' },
  ],
  
  // API endpoints
  endpoints: {
    health: '/health',
    swagger: '/swagger/index.html',
    auth: {
      register: '/api/v1/auth/register',
      login: '/api/v1/auth/login',
      logout: '/api/v1/auth/user/logout',
      profile: '/api/v1/auth/user/profile',
      updateProfile: '/api/v1/auth/user/profile',
      changePassword: '/api/v1/auth/user/change-password',
      deleteUser: '/api/v1/auth/user',
      googleAuth: '/api/v1/auth/google',
      googleCallback: '/api/v1/auth/google/callback',
      verify: '/api/v1/auth/verify',
      forgotPassword: '/api/v1/auth/forgot-password',
      resetPassword: '/api/v1/auth/reset-password',
    },
    notification: '/api/v1/notification',
    notificationWs: '/api/v1/notification/ws',
  },
  
  // Request timeouts
  timeouts: {
    default: 30000,      // 30 seconds
    long: 60000,         // 60 seconds
    short: 10000,        // 10 seconds
  },
  
  // Think times (in seconds)
  thinkTimes: {
    min: 1,
    max: 5,
    average: 2,
  },
};

// Helper function to get base URL
export function getBaseUrl() {
  const env = __ENV.ENV || 'local';
  return __ENV.BASE_URL || config.environments[env];
}

// Helper function to get random test user
export function getRandomTestUser() {
  return config.testUsers[Math.floor(Math.random() * config.testUsers.length)];
}

// Helper function to generate random email
export function generateRandomEmail(prefix = 'test') {
  return `${prefix}-${Date.now()}-${Math.floor(Math.random() * 10000)}@example.com`;
}

// Helper function to get auth headers
export function getAuthHeaders(token) {
  return {
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${token}`,
  };
}

// Helper function to get standard headers
export function getStandardHeaders() {
  return {
    'Content-Type': 'application/json',
  };
}
