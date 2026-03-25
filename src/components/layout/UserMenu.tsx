import { useState, useRef, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { LogIn, LogOut, User, ChevronDown, Shield, Settings } from 'lucide-react';
import { useAuth, useAuthStore } from '../../hooks/useAuth';
import LazyImage from '../shared/LazyImage';
import styles from './UserMenu.module.css';

const UserMenu: React.FC = () => {
  const { user, isAuthenticated, logout } = useAuth();
  const isModerator = user?.role === 'moderator' || user?.role === 'admin';
  const resetAuth = useAuthStore((s) => s.reset);
  const [open, setOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const handleLogin = () => {
    resetAuth(); 
    window.location.href = '/api/v1/auth/discord';
  };

  if (!isAuthenticated || !user) {
    return (
      <button className={styles.loginButton} onClick={handleLogin}>
        <LogIn size={16} />
        <span>Login</span>
      </button>
    );
  }

  return (
    <div className={styles.wrapper} ref={menuRef}>
      <button className={styles.trigger} onClick={() => setOpen(!open)}>
        <div className={styles.avatarWrapper}>
          {user.avatar ? (
            <LazyImage src={user.avatar} alt={user.displayName} className={styles.avatar} containerClassName={styles.avatar} spinnerSize={14} />
          ) : (
            <div className={styles.avatarFallback}>
              {user.displayName.charAt(0).toUpperCase()}
            </div>
          )}
          {isModerator && (
            <div className={styles.modBadge} title="Moderator">
              <Shield size={10} />
            </div>
          )}
        </div>
        <span className={styles.username}>{user.displayName}</span>
        <ChevronDown size={14} className={open ? styles.chevronOpen : styles.chevron} />
      </button>

      {open && (
        <>
          <div className={styles.backdrop} onClick={() => setOpen(false)} />
          <div className={styles.dropdown}>
            <div className={styles.sheetHeader}>
              <div className={styles.sheetAvatar}>
                {user.avatar ? (
                  <LazyImage
                    src={user.avatar}
                    alt={user.displayName}
                    className={styles.sheetAvatarImg}
                    containerClassName={styles.sheetAvatarImg}
                    spinnerSize={18}
                  />
                ) : (
                  <div className={styles.sheetAvatarFallback}>
                    {user.displayName.charAt(0).toUpperCase()}
                  </div>
                )}
                {isModerator && (
                  <div className={styles.sheetModBadge}>
                    <Shield size={12} />
                  </div>
                )}
              </div>
              <div>
                <p className={styles.sheetName}>{user.displayName}</p>
                <p className={styles.sheetUsername}>@{user.username}</p>
              </div>
            </div>
            <Link
              to={`/user/${user.discordId}`}
              className={styles.dropdownItem}
              onClick={() => setOpen(false)}
            >
              <User size={16} />
              <span>My Profile</span>
            </Link>
            <Link
              to="/settings"
              className={styles.dropdownItem}
              onClick={() => setOpen(false)}
            >
              <Settings size={16} />
              <span>Settings</span>
            </Link>
            <div className={styles.dropdownDivider} />
            <button
              className={`${styles.dropdownItem} ${styles.logoutItem}`}
              onClick={() => {
                logout();
                setOpen(false);
              }}
            >
              <LogOut size={16} />
              <span>Logout</span>
            </button>
          </div>
        </>
      )}
    </div>
  );
};

export default UserMenu;
