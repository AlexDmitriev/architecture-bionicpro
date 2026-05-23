import React from 'react';
import ConsentPage from './components/ConsentPage';
import ReportPage from './components/ReportPage';

const App: React.FC = () => {
  const path = window.location.pathname;

  if (path === '/consent' || path.startsWith('/consent/')) {
    return <ConsentPage />;
  }

  return (
    <div className="App">
      <ReportPage />
    </div>
  );
};

export default App;
