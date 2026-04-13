import { BrowserRouter, Routes, Route } from 'react-router';
import Layout from './components/layout/Layout';
import Home from './pages/home/Home';
import Characters from './pages/characters/Characters';
import CharacterDetail from './pages/characters/CharacterDetail';
import Worldbooks from './pages/worldbooks/Worldbooks';
import WorldbookDetail from './pages/worldbooks/WorldbookDetail';
import UserProfile from './pages/user/UserProfile';
import Themes from './pages/themes/Themes';
import Presets from './pages/presets/Presets';
import Settings from './pages/settings/Settings';
import Leaderboard from './pages/leaderboard/Leaderboard';

function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<Layout />}>
          <Route index element={<Home />} />
          <Route path="characters" element={<Characters />} />
          <Route path="characters/:id" element={<CharacterDetail />} />
          <Route path="worldbooks" element={<Worldbooks />} />
          <Route path="worldbooks/:id" element={<WorldbookDetail />} />
          <Route path="themes" element={<Themes />} />
          <Route path="presets" element={<Presets />} />
          <Route path="user/:discordId" element={<UserProfile />} />
          <Route path="settings" element={<Settings />} />
          <Route path="leaderboard" element={<Leaderboard />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}

export default App;
