import { useState, useEffect } from 'react';
import { Spin } from 'antd';
import Login from './pages/Login';
import Dashboard from './pages/Dashboard';

// Mock functions for development (Wails bindings will override these)
const mockIsLoggedIn = async () => {
  const token = localStorage.getItem('token');
  return !!token;
};

// Use Wails bindings if available, otherwise use mocks
const IsLoggedIn = (window as any).go?.main?.App?.IsLoggedIn || mockIsLoggedIn;

function App() {
  const [loading, setLoading] = useState(true);
  const [isLoggedIn, setIsLoggedIn] = useState(false);

  useEffect(() => {
    checkAuth();
  }, []);

  const checkAuth = async () => {
    try {
      const loggedIn = await IsLoggedIn();
      setIsLoggedIn(loggedIn);
    } catch {
      setIsLoggedIn(false);
    } finally {
      setLoading(false);
    }
  };

  const handleLogin = () => {
    setIsLoggedIn(true);
  };

  const handleLogout = () => {
    setIsLoggedIn(false);
  };

  if (loading) {
    return (
      <div style={{
        display: 'flex',
        justifyContent: 'center',
        alignItems: 'center',
        height: '100vh'
      }}>
        <Spin size="large" />
      </div>
    );
  }

  if (!isLoggedIn) {
    return <Login onLogin={handleLogin} />;
  }

  return <Dashboard onLogout={handleLogout} />;
}

export default App;
