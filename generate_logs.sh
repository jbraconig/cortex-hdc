#!/bin/bash

LOG_FILE="test-data/p2p/node1.log"

# Asegurarnos de que el directorio exista
mkdir -p "$(dirname "$LOG_FILE")"

echo "Generando logs en $LOG_FILE..."
echo "Presiona Ctrl+C para detener el script."

# Arrays con variedad de mensajes sanos (INFO)
HEALTHY_MSGS=(
    "INFO [P2P] Operación exitosa. Nodo sincronizado correctamente."
    "INFO [Network] Descubrimiento de peers completado. Encontrados 18 peers."
    "INFO [P2P] Conexión establecida con éxito con el peer 0x4f1a."
    "INFO [Blockchain] Sincronización de bloques en progreso (15%)."
    "INFO [DB] Estado guardado correctamente en la base de datos local."
    "INFO [P2P] Mensaje de latido (heartbeat) recibido del peer 0x8b32."
    "INFO [System] Uso de memoria estable en 215MB."
    "INFO [Network] Latencia con el peer 0x8b32 es de 34ms."
    "INFO [Blockchain] Nuevo bloque 593021 validado y añadido a la cadena local."
    "INFO [System] Ciclo de validación completado en 45ms."
    "INFO [DB] Limpieza de caché completada exitosamente."
    "INFO [P2P] Compartiendo lista de peers activos."
)

# Arrays con variedad de anomalías (WARNING / ERROR)
ANOMALY_MSGS=(
    "ERROR [P2P] Fallo de conexión. Timeout excedido al contactar peer 0x9b11."
    "ERROR [P2P] Conexión rechazada. Protocolo incompatible con el peer 0x11a0."
    "WARNING [P2P] Alta latencia detectada en la red (850ms)."
    "ERROR [Blockchain] Error al validar el bloque 593050. Hash incorrecto."
    "ERROR [Blockchain] Desincronización detectada. Re-descargando últimos bloques."
    "WARNING [Network] Peer 0x9b11 no responde. Reintentando..."
    "WARNING [DB] Operación de escritura lenta. Tiempo de respuesta: 1250ms."
)

while true; do
    # Format like: 2026-06-22T13:05:33-05:00
    TIMESTAMP=$(date "+%Y-%m-%dT%H:%M:%S%z" | sed 's/\(..\)$/:\1/')
    RANDOM_NUM=$RANDOM
    
    # 80% probabilidad de éxito (logs sanos), 20% de anomalías
    if [ $((RANDOM_NUM % 10)) -lt 8 ]; then
        # Seleccionar log sano aleatorio
        RANDOM_IDX=$((RANDOM_NUM % ${#HEALTHY_MSGS[@]}))
        echo "$TIMESTAMP ${HEALTHY_MSGS[$RANDOM_IDX]}" >> "$LOG_FILE"
    else
        # Seleccionar log de anomalía aleatorio
        RANDOM_IDX=$((RANDOM_NUM % ${#ANOMALY_MSGS[@]}))
        echo "$TIMESTAMP ${ANOMALY_MSGS[$RANDOM_IDX]}" >> "$LOG_FILE"
    fi
    
    # Esperar un tiempo aleatorio (0, 1 o 2 segundos) entre cada log
    sleep $((RANDOM_NUM % 3))
done
