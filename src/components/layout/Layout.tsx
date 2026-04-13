import { useState } from 'react';
import { Outlet, Link, useLocation } from 'react-router-dom';
import { Home, Users, Book, Palette, Sparkles, User, LogIn, Settings, LogOut, Trophy } from 'lucide-react';
import clsx from 'clsx';
import { useAuth } from '../../hooks/useAuth';
import UserMenu from './UserMenu';
import styles from './Layout.module.css';

const Layout = () => {
  const location = useLocation();
  const { user, isAuthenticated, logout } = useAuth();
  const [accountOpen, setAccountOpen] = useState(false);

  const isActive = (path: string) => {
    if (path === '/') return location.pathname === '/';
    return location.pathname.startsWith(path);
  };

  return (
    <div className={styles.shell} data-studio="shell">
      <header className={styles.header} data-studio="header">
        <div className={styles.headerInner}>
          <Link to="/" className={styles.brand}>
            <Sparkles className={styles.brandIcon} size={22} strokeWidth={2.5} />
            <span className={styles.brandText}>LumiHub</span>
          </Link>

          <nav className={styles.nav} data-studio="nav">
            <Link
              to="/"
              className={clsx(styles.navItem, isActive('/') && styles.navItemActive)}
            >
              <Home size={18} strokeWidth={isActive('/') ? 2.5 : 2} />
              <span>Discover</span>
            </Link>

            <Link
              to="/characters"
              className={clsx(styles.navItem, isActive('/characters') && styles.navItemActive)}
            >
              <Users size={18} strokeWidth={isActive('/characters') ? 2.5 : 2} />
              <span>Characters</span>
            </Link>

            <Link
              to="/worldbooks"
              className={clsx(styles.navItem, isActive('/worldbooks') && styles.navItemActive)}
            >
              <Book size={18} strokeWidth={isActive('/worldbooks') ? 2.5 : 2} />
              <span>Worldbooks</span>
            </Link>

            <Link
              to="/themes"
              className={clsx(styles.navItem, isActive('/themes') && styles.navItemActive)}
            >
              <Palette size={18} strokeWidth={isActive('/themes') ? 2.5 : 2} />
              <span>Themes</span>
            </Link>

            <Link
              to="/presets"
              className={clsx(styles.navItem, isActive('/presets') && styles.navItemActive)}
            >
              <Sparkles size={18} strokeWidth={isActive('/presets') ? 2.5 : 2} />
              <span>Presets</span>
            </Link>

            <Link
              to="/leaderboard"
              className={clsx(styles.navItem, isActive('/leaderboard') && styles.navItemActive)}
            >
              <Trophy size={18} strokeWidth={isActive('/leaderboard') ? 2.5 : 2} />
              <span>Leaderboard</span>
            </Link>

            {isAuthenticated && user ? (
              <button
                type="button"
                className={clsx(styles.navItem, styles.navAccount, accountOpen && styles.navItemActive)}
                onClick={() => setAccountOpen(v => !v)}
              >
                {user.avatar ? (
                  <img src={user.avatar} alt="" className={styles.navAvatar} />
                ) : (
                  <User size={18} />
                )}
                <span>Me</span>
              </button>
            ) : (
              <a
                href="/api/v1/auth/discord"
                className={clsx(styles.navItem, styles.navAccount)}
              >
                <LogIn size={18} />
                <span>Login</span>
              </a>
            )}
          </nav>

          <div className={styles.headerRight}>
            <UserMenu />
          </div>
        </div>
      </header>

      <main className={styles.main} data-studio="main">
        <Outlet />
      </main>

      {accountOpen && isAuthenticated && user && (
        <>
          <div className={styles.sheetBackdrop} onClick={() => setAccountOpen(false)} />
          <div className={styles.sheet}>
            <div className={styles.sheetHeader}>
              {user.avatar ? (
                <img src={user.avatar} alt="" className={styles.sheetAvatar} />
              ) : (
                <div className={styles.sheetAvatarFallback}>
                  {user.displayName.charAt(0).toUpperCase()}
                </div>
              )}
              <div>
                <p className={styles.sheetName}>{user.displayName}</p>
                <p className={styles.sheetHandle}>@{user.username}</p>
              </div>
            </div>
            <div className={styles.sheetDivider} />
            <Link
              to={`/user/${user.discordId}`}
              className={styles.sheetItem}
              onClick={() => setAccountOpen(false)}
            >
              <User size={18} />
              <span>My Profile</span>
            </Link>
            <Link
              to="/settings"
              className={styles.sheetItem}
              onClick={() => setAccountOpen(false)}
            >
              <Settings size={18} />
              <span>Settings</span>
            </Link>
            <div className={styles.sheetDivider} />
            <button
              className={`${styles.sheetItem} ${styles.sheetLogout}`}
              onClick={() => {
                logout();
                setAccountOpen(false);
              }}
            >
              <LogOut size={18} />
              <span>Log Out</span>
            </button>
          </div>
        </>
      )}
    </div>
  );
};

export default Layout;
