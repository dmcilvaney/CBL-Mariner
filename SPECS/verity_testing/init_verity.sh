IMAGE=${1:-verity.img}
HASHTREE=${IMAGE%.img}.hashtree
LOG=${IMAGE%.img}.log
ROOTHASH=${IMAGE%.img}.roothash
FEC=${IMAGE%.img}.fec

truncate $IMAGE --size 2G
mkfs.ext4 verity.img
losetup -D
export LOOP=$(losetup -f)
losetup $LOOP $IMAGE
veritysetup --fec-device=$FEC format $LOOP $HASHTREE --debug | tee $LOG
export ROOT_HASH=$(grep ^"Root hash" $LOG | cut -f2 | tr -d "\n")
echo $ROOT_HASH > $ROOTHASH
