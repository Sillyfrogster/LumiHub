import styles from "./PaperGrain.module.css";

/** Paper texture laid over the whole page */
export function PaperGrain() {
  return <div className={styles.grain} aria-hidden />;
}
