{-# LANGUAGE DeriveGeneric #-}
import Control.Monad
import Control.Monad.Conc.Class
import qualified Data.Map.Strict as M
import Data.List (nub, sortOn)
import Data.Maybe (fromMaybe)
import GHC.Generics (Generic)
import System.Exit
import Test.DejaFu

type Proc = Int
type DiskId = Int
type Ballot = Int

data Val
  = NoInput
  | In Int
  deriving (Eq, Ord, Generic)

instance Show Val where
  show NoInput = "NotAnInput"
  show (In i)  = "in" ++ show i

data Block = Block
  { mbal :: Ballot
  , bal  :: Ballot
  , inp  :: Val
  } deriving (Eq, Ord, Generic)

instance Show Block where
  show (Block m b v) =
    "[mbal=" ++ show m ++ ", bal=" ++ show b ++ ", inp=" ++ show v ++ "]"

data Config = Config
  { cfgName               :: String
  , nProc                 :: Int
  , inputsDomain          :: [Val]
  , diskDomain            :: [DiskId]
  , ballotCountPerProcess :: Int
  , maxAttempts           :: Int
  } deriving Show

data Decision = Decision
  { decProc     :: Proc
  , decInput    :: Val
  , decOutput   :: Maybe Val
  , decAttempts :: Int
  } deriving (Eq, Show)

initBlock :: Block
initBlock = Block 0 0 NoInput

smallConfig :: Config
smallConfig =
  Config
    { cfgName = "small"
    , nProc = 2
    , inputsDomain = [In 1, In 2]
    , diskDomain = [1, 2]
    , ballotCountPerProcess = 1
    , maxAttempts = 10
    }

mediumConfig :: Config
mediumConfig =
  Config
    { cfgName = "medium"
    , nProc = 2
    , inputsDomain = [In 1, In 2]
    , diskDomain = [1, 2]
    , ballotCountPerProcess = 2
    , maxAttempts = 10
    }

mainConfig :: Config
mainConfig =
  Config
    { cfgName = "main"
    , nProc = 3
    , inputsDomain = [In 1, In 2]
    , diskDomain = [1, 2]
    , ballotCountPerProcess = 2
    , maxAttempts = 15
    }

procs :: Config -> [Proc]
procs c = [1 .. nProc c]

majoritySize :: Config -> Int
majoritySize c = nProc c `div` 2 + 1

majorityDisks :: Config -> [DiskId]
majorityDisks c = take (majoritySize c) (diskDomain c)

ballotsOf :: Config -> Proc -> [Ballot]
ballotsOf c p =
  let start = p * ballotCountPerProcess c
  in [start .. start + ballotCountPerProcess c - 1]

type DiskState = M.Map Proc Block
type Disk m = MVar m DiskState
newtype DiskTable m = DiskTable (M.Map DiskId (Disk m))

newDisks :: MonadConc m => Config -> m (DiskTable m)
newDisks c = do
  pairs <- forM (diskDomain c) $ \d -> do
    mv <- newMVar (M.fromList [(p, initBlock) | p <- procs c])
    return (d, mv)
  return (DiskTable (M.fromList pairs))

lookupDisk :: DiskTable m -> DiskId -> Disk m
lookupDisk (DiskTable disks) d =
  fromMaybe (error ("missing disk " ++ show d)) (M.lookup d disks)

readDiskBlock :: MonadConc m => DiskTable m -> DiskId -> Proc -> m Block
readDiskBlock disks d p = do
  let mv = lookupDisk disks d
  st <- takeMVar mv
  putMVar mv st
  return $ fromMaybe initBlock (M.lookup p st)

writeDiskBlock :: MonadConc m => DiskTable m -> DiskId -> Proc -> Block -> m ()
writeDiskBlock disks d p blk = do
  let mv = lookupDisk disks d
  st <- takeMVar mv
  putMVar mv (M.insert p blk st)

writeMajorityBlockBuggy :: MonadConc m => Config -> DiskTable m -> Proc -> Block -> m ()
writeMajorityBlockBuggy c disks p blk = do
  let ds =
        if odd p
          then majorityDisks c
          else reverse (majorityDisks c)

      acquire [] acc =
        return acc

      acquire (d:rest) acc = do
        let mv = lookupDisk disks d
        st <- takeMVar mv
        acquire rest ((d, mv, st) : acc)

      release [] =
        return ()

      release ((d, mv, st):rest) = do
        putMVar mv (M.insert p blk st)
        release rest

  locked <- acquire ds []
  release locked

inputsForRun :: Config -> Int -> M.Map Proc Val
inputsForRun c runId =
  let vals = inputsDomain c
      pick p = vals !! ((runId + p) `mod` length vals)
  in M.fromList [(p, pick p) | p <- procs c]

highestAcceptedValue :: Val -> [Block] -> Val
highestAcceptedValue ownInput blocks =
  let accepted = filter (\b -> inp b /= NoInput && bal b > 0) blocks
  in case accepted of
       [] -> ownInput
       xs ->
         let maxBal = maximum (map bal xs)
         in inp (head [b | b <- sortOn bal xs, bal b == maxBal])

phase0ReadOwnBlocks :: MonadConc m => Config -> DiskTable m -> Proc -> m Block
phase0ReadOwnBlocks c disks p = do
  ownBlocks <- forM (majorityDisks c) $ \d ->
    readDiskBlock disks d p

  let maxAccepted = maximum (map bal ownBlocks)
      chosenBlock = head [b | b <- ownBlocks, bal b == maxAccepted]

  return chosenBlock

runBallotAttempt
  :: MonadConc m
  => Config
  -> DiskTable m
  -> Proc
  -> Val
  -> Ballot
  -> m (Maybe Val)
runBallotAttempt c disks p ownInput b = do
  startBlock <- phase0ReadOwnBlocks c disks p

  let prepareBlock = startBlock { mbal = b }

  writeMajorityBlockBuggy c disks p prepareBlock

  phase1Reads <- fmap concat $ forM (majorityDisks c) $ \d ->
    forM [q | q <- procs c, q /= p] $ \q ->
      readDiskBlock disks d q

  let higherMbalSeen = any (\blk -> mbal blk > b) phase1Reads

  if higherMbalSeen
    then return Nothing
    else do
      let chosenVal = highestAcceptedValue ownInput (prepareBlock : phase1Reads)
          acceptBlock = Block b b chosenVal

      writeMajorityBlockBuggy c disks p acceptBlock

      phase2Reads <- fmap concat $ forM (majorityDisks c) $ \d ->
        forM [q | q <- procs c, q /= p] $ \q ->
          readDiskBlock disks d q

      let higherAfterAccept = any (\blk -> mbal blk > b) phase2Reads

      if higherAfterAccept
        then return Nothing
        else return (Just chosenVal)

proposerProcess
  :: MonadConc m
  => Config
  -> DiskTable m
  -> Proc
  -> Val
  -> m Decision
proposerProcess c disks p ownInput =
  tryAttempts 1 (cycle (ballotsOf c p))
  where
    tryAttempts attempt _
      | attempt > maxAttempts c =
          return (Decision p ownInput Nothing (attempt - 1))

    tryAttempts attempt (b:bs) = do
      r <- runBallotAttempt c disks p ownInput b
      case r of
        Just v ->
          return (Decision p ownInput (Just v) attempt)

        Nothing ->
          tryAttempts (attempt + 1) bs

    tryAttempts _ [] =
      return (Decision p ownInput Nothing 0)

runDiskPaxos :: MonadConc m => Config -> Int -> m [Decision]
runDiskPaxos c runId = do
  disks <- newDisks c

  let inputMap = inputsForRun c runId
      start p = do
        done <- newEmptyMVar
        let ownInput = fromMaybe NoInput (M.lookup p inputMap)
        _ <- fork $ proposerProcess c disks p ownInput >>= putMVar done
        return done

  doneVars <- mapM start (procs c)
  mapM takeMVar doneVars

agreementOK :: [Decision] -> Bool
agreementOK ds =
  case nub [v | Decision _ _ (Just v) _ <- ds] of
    []  -> True
    [_] -> True
    _   -> False

validityOK :: Config -> [Decision] -> Bool
validityOK c ds =
  all (`elem` inputsDomain c) [v | Decision _ _ (Just v) _ <- ds]

checkedRun :: Config -> Int -> ConcIO Bool
checkedRun c runId = do
  ds <- runDiskPaxos c runId
  return (agreementOK ds && validityOK c ds)

runCheck :: String -> ConcIO Bool -> IO Bool
runCheck label action = do
  putStrLn ("Checking " ++ label ++ " with DejaFu...")
  ok <- autocheck action
  putStrLn ""
  return ok

main :: IO ()
main = do
  ok1 <- runCheck "small, run 1" (checkedRun smallConfig 1)
  ok2 <- runCheck "small, run 2" (checkedRun smallConfig 2)
  ok3 <- runCheck "medium, run 1" (checkedRun mediumConfig 1)

  if ok1 && ok2 && ok3
    then putStrLn "UNEXPECTED: buggy lock-order implementation passed."
    else exitFailure
