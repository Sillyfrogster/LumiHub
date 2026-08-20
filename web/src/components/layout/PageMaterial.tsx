import styles from "./PageMaterial.module.css";

export function PageMaterial() {
  return (
    <div className={styles.material} aria-hidden="true">
      <div className={styles.paper} />
      <div className={styles.night} />
    </div>
  );
}
